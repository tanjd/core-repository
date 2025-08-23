import { readdir, readFile, copyFile, mkdir } from "fs/promises";
import { join } from "path";
import { createHash } from "crypto";

const TAKEOUT_DIR = "/workspace/Takeout/saved";
const TARGET_DIR = join(process.cwd(), "data/food-maps");

async function getFileHash(filePath: string): Promise<string> {
  const content = await readFile(filePath, "utf-8");
  return createHash("md5").update(content, "utf-8").digest("hex");
}

async function syncTakeoutFiles() {
  // Ensure target directory exists
  await mkdir(TARGET_DIR, { recursive: true });

  // Get all files from Takeout
  const takeoutFiles = await readdir(TAKEOUT_DIR);
  const foodMapFiles = takeoutFiles.filter((file) =>
    file.endsWith("-food.csv"),
  );

  console.log(`Found ${foodMapFiles.length} food map files in Takeout`);

  let copied = 0;
  let skipped = 0;
  let errors: string[] = [];

  for (const file of foodMapFiles) {
    try {
      const sourcePath = join(TAKEOUT_DIR, file);
      // Keep the same filename format
      const targetFile = file;
      const targetPath = join(TARGET_DIR, targetFile);

      // Check if target file exists
      try {
        const sourceHash = await getFileHash(sourcePath);
        let targetHash = "";

        try {
          targetHash = await getFileHash(targetPath);
        } catch (err) {
          // File doesn't exist, that's fine
        }

        if (sourceHash !== targetHash) {
          await copyFile(sourcePath, targetPath);
          console.log(`Copied: ${file} -> ${targetFile}`);
          copied++;
        } else {
          console.log(`Skipped (identical): ${file}`);
          skipped++;
        }
      } catch (err) {
        // If target doesn't exist, just copy
        await copyFile(sourcePath, targetPath);
        console.log(`Copied new file: ${file} -> ${targetFile}`);
        copied++;
      }
    } catch (error) {
      const errMsg = error instanceof Error ? error.message : String(error);
      console.error(`Error processing ${file}:`, errMsg);
      errors.push(`Error processing ${file}: ${errMsg}`);
    }
  }

  console.log("\nSync Results:");
  console.log(`- Files copied: ${copied}`);
  console.log(`- Files skipped: ${skipped}`);

  if (errors.length > 0) {
    console.log("\nErrors:");
    errors.forEach((error) => console.error(`- ${error}`));
  }
}

// Run if called directly
if (require.main === module) {
  syncTakeoutFiles().catch(console.error);
}

export { syncTakeoutFiles };
