import { readdir, readFile } from "fs/promises";
import { join } from "path";
import { StorageManager } from "./storage";
import { parseCsvContent } from "../csv-parser";
import { cityToCountry } from "../../../../../apps/food-maps/scripts/city-country-map";

async function importCsvFiles() {
  const csvDir = join(process.cwd(), "data/food-maps");
  const dbPath = join(process.cwd(), "data/food-maps.db");

  try {
    // Initialize database
    console.log("Initializing SQLite database...");
    const storage = new StorageManager(dbPath);
    await storage.init();
    console.log("Database initialized successfully.");

    // Read CSV files
    console.log("Reading CSV files...");
    const files = await readdir(csvDir);
    const foodMapFiles = files.filter((file) => file.endsWith("-food.csv"));

    let totalAdded = 0;
    let totalUpdated = 0;
    const errors: string[] = [];

    for (const file of foodMapFiles) {
      try {
        const content = await readFile(join(csvDir, file), "utf-8");
        const city = file
          .replace(/-food\.csv$/, "") // New format
          .replace(/-/g, " "); // Convert dashes to spaces
        const country = cityToCountry[city] || "Unknown";

        console.log(`Processing ${file}...`);
        const locations = parseCsvContent(content, city, country);
        const result = await storage.addLocations(locations);

        totalAdded += result.added;
        totalUpdated += result.updated;
        errors.push(...result.errors);

        console.log(`- Added: ${result.added}`);
        console.log(`- Updated: ${result.updated}`);
        console.log(`- Skipped: ${result.skipped}`);
        console.log();
      } catch (error) {
        const errMsg = error instanceof Error ? error.message : String(error);
        console.error(`Error processing ${file}:`, errMsg);
        errors.push(`Error processing ${file}: ${errMsg}`);
      }
    }

    console.log("\nImport Results:");
    console.log(`- Total locations added: ${totalAdded}`);
    console.log(`- Total locations updated: ${totalUpdated}`);

    if (errors.length > 0) {
      console.log("\nErrors:");
      errors.forEach((error) => console.error(`- ${error}`));
    }

    await storage.close();
    console.log("\nImport complete!");
  } catch (error) {
    console.error("Import failed:", error);
    process.exit(1);
  }
}

// Run if called directly
if (require.main === module) {
  importCsvFiles().catch(console.error);
}

export { importCsvFiles };
