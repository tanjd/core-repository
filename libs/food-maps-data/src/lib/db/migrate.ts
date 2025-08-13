import { readFile } from "fs/promises";
import { join } from "path";
import { StorageManager } from "./storage";
import { FoodLocation } from "../types";

async function migrateToSqlite() {
  const jsonPath = join(process.cwd(), "data/master.json");
  const dbPath = join(process.cwd(), "data/food-maps.db");

  try {
    // Read existing JSON data
    console.log("Reading master.json...");
    const content = await readFile(jsonPath, "utf-8");
    const locations: FoodLocation[] = JSON.parse(content);

    // Initialize new database
    console.log("Initializing SQLite database...");
    const storage = new StorageManager(dbPath);
    await storage.init();

    // Import data
    console.log("Importing locations...");
    const result = await storage.addLocations(locations);

    console.log("\nMigration Results:");
    console.log(`- Locations added: ${result.added}`);
    console.log(`- Locations updated: ${result.updated}`);
    console.log(`- Locations skipped: ${result.skipped}`);

    if (result.errors.length > 0) {
      console.log("\nErrors:");
      result.errors.forEach((error) => console.error(`- ${error}`));
    }

    await storage.close();
    console.log("\nMigration complete!");
  } catch (error) {
    console.error("Migration failed:", error);
    process.exit(1);
  }
}

// Run if called directly
if (require.main === module) {
  migrateToSqlite().catch(console.error);
}

export { migrateToSqlite };
