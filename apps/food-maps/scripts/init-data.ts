import {
  StorageManager,
  parseCsvContent,
} from "../../../libs/food-maps-data/src";
import { cityToCountry } from "./city-country-map";
import { join } from "path";
import { readdir, readFile, mkdir } from "fs/promises";

async function ensureDataDir() {
  const dataDir = join(process.cwd(), "data");
  await mkdir(dataDir, { recursive: true });
}

async function initializeData() {
  await ensureDataDir();

  const storage = new StorageManager(join(process.cwd(), "data/master.json"));
  await storage.load();

  const csvDir = join(process.cwd(), "data/food-maps");
  const files = await readdir(csvDir);

  let totalAdded = 0;
  let totalUpdated = 0;
  let errors: string[] = [];

  for (const file of files) {
    if (!file.endsWith(".csv")) continue;

    const content = await readFile(join(csvDir, file), "utf-8");
    const city = file.replace(/\s*\(Food\)\.csv$/, "");
    const country = cityToCountry[city] || "Unknown";

    try {
      const locations = parseCsvContent(content, city, country);
      const result = await storage.addLocations(locations);
      totalAdded += result.added;
      totalUpdated += result.updated;
      errors.push(...result.errors);

      console.log(`Processed ${file}:`);
      console.log(`- Added: ${result.added}`);
      console.log(`- Updated: ${result.updated}`);
      console.log(`- Skipped: ${result.skipped}`);
      console.log();
    } catch (error) {
      console.error(`Error processing ${file}:`, error);
      errors.push(`Error processing ${file}: ${error.message}`);
    }
  }

  console.log("\nFinal Results:");
  console.log(`Total Added: ${totalAdded}`);
  console.log(`Total Updated: ${totalUpdated}`);

  if (errors.length > 0) {
    console.log("\nErrors:");
    errors.forEach((error) => console.error(`- ${error}`));
  }
}

initializeData().catch(console.error);
