import { mkdir } from "fs/promises";
import { join } from "path";
import { syncTakeoutFiles } from "./sync-takeout";
import { importCsvFiles } from "../../../libs/food-maps-data/src/lib/db/import-csv";

async function ensureDataDir() {
  const dataDir = join(process.cwd(), "data");
  await mkdir(dataDir, { recursive: true });
}

async function initializeData() {
  await ensureDataDir();

  // First sync files from Takeout if any
  await syncTakeoutFiles();

  // Import CSV files into SQLite
  await importCsvFiles();
}

// Run if called directly
if (require.main === module) {
  initializeData().catch(console.error);
}

export { initializeData };
