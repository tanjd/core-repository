import { ApiClient, parseCsvContent } from "@tanjd/food-maps-data";
import path from "node:path";
import { readdir, readFile } from "node:fs/promises";
import { cityToCountry } from "../../../scripts/city-country-map";

async function processFiles() {
  try {
    const apiClient = new ApiClient();

    // Check if backend is available
    const isHealthy = await apiClient.healthCheck();
    if (!isHealthy) {
      throw new Error("Backend is not available");
    }

    const csvDir = path.join(process.cwd(), "../../data/food-maps");
    console.log("Reading CSV files from:", csvDir);

    const files = await readdir(csvDir);

    let totalAdded = 0;
    let totalUpdated = 0;
    let errors: string[] = [];

    console.log("Found files:", files);

    for (const file of files) {
      if (!file.endsWith(".csv")) continue;

      const content = await readFile(path.join(csvDir, file), "utf-8");
      // Handle both formats: "City (Food).csv" and "city-food.csv"
      const city = file
        .replace(/\s*\(Food\)\.csv$/, "") // Old format
        .replace(/-food\.csv$/, "") // New format
        .replace(/-/g, " "); // Convert dashes to spaces
      const country = cityToCountry[city] || "Unknown";

      try {
        const locations = parseCsvContent(content, city, country);
        const result = await apiClient.addLocations(locations);
        totalAdded += result.added;
        totalUpdated += result.updated;
        errors.push(...result.errors);
      } catch (error) {
        if (error instanceof Error) {
          errors.push(`Error processing ${file}: ${error.message}`);
        } else {
          errors.push(`Error processing ${file}: ${String(error)}`);
        }
      }
    }

    return { totalAdded, totalUpdated, errors };
  } catch (error) {
    console.error("Error in processFiles:", error);
    return {
      totalAdded: 0,
      totalUpdated: 0,
      errors: [
        `Failed to process files: ${
          error instanceof Error ? error.message : String(error)
        }`,
      ],
    };
  }
}

export default async function UploadPage() {
  const result = await processFiles();

  return (
    <main className="container mx-auto px-4 py-8">
      <h1 className="text-4xl font-bold mb-8">Upload Results</h1>

      <div className="bg-white rounded-lg shadow p-6">
        <div className="grid grid-cols-2 gap-4 mb-6">
          <div className="p-4 bg-green-50 rounded">
            <p className="text-lg font-medium text-green-700">Added</p>
            <p className="text-3xl font-bold text-green-800">
              {result.totalAdded}
            </p>
          </div>
          <div className="p-4 bg-blue-50 rounded">
            <p className="text-lg font-medium text-blue-700">Updated</p>
            <p className="text-3xl font-bold text-blue-800">
              {result.totalUpdated}
            </p>
          </div>
        </div>

        {result.errors.length > 0 && (
          <div className="mt-6">
            <h2 className="text-xl font-semibold mb-3">Errors</h2>
            <ul className="space-y-2">
              {result.errors.map((error, i) => (
                <li key={i} className="text-red-600 bg-red-50 p-3 rounded">
                  {error}
                </li>
              ))}
            </ul>
          </div>
        )}
      </div>
    </main>
  );
}
