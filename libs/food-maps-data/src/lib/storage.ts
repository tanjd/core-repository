import { writeFile, readFile } from "fs/promises";
import { FoodLocation, CsvUploadResult, LocationGroup } from "./types";
import { mergeFoodLocations } from "./csv-parser";

export class StorageManager {
  private masterFilePath: string;
  private locations: FoodLocation[] = [];

  constructor(masterFilePath: string) {
    this.masterFilePath = masterFilePath;
  }

  async load(): Promise<void> {
    try {
      const content = await readFile(this.masterFilePath, "utf-8");
      this.locations = JSON.parse(content);
    } catch (error) {
      // If file doesn't exist, start with empty array
      this.locations = [];
    }
  }

  async save(): Promise<void> {
    await writeFile(
      this.masterFilePath,
      JSON.stringify(this.locations, null, 2),
      "utf-8",
    );
  }

  async addLocations(newLocations: FoodLocation[]): Promise<CsvUploadResult> {
    const result = mergeFoodLocations(this.locations, newLocations);

    // Update the locations array with merged results
    const locationsMap = new Map(this.locations.map((loc) => [loc.id, loc]));
    newLocations.forEach((loc) => locationsMap.set(loc.id, loc));
    this.locations = Array.from(locationsMap.values());

    await this.save();
    return result;
  }

  getLocationsByCountry(): LocationGroup[] {
    const groupedByCountry = new Map<string, Map<string, FoodLocation[]>>();

    // Group locations by country and city
    for (const location of this.locations) {
      if (!groupedByCountry.has(location.country)) {
        groupedByCountry.set(location.country, new Map());
      }
      const countryGroup = groupedByCountry.get(location.country)!;

      if (!countryGroup.has(location.city)) {
        countryGroup.set(location.city, []);
      }
      countryGroup.get(location.city)!.push(location);
    }

    // Convert to LocationGroup array
    return Array.from(groupedByCountry.entries()).map(([country, cities]) => ({
      country,
      cities: Array.from(cities.entries()).map(([name, locations]) => ({
        name,
        locations,
      })),
      totalLocations: Array.from(cities.values()).reduce(
        (sum, locs) => sum + locs.length,
        0,
      ),
    }));
  }

  getLocationsByCity(country: string, city: string): FoodLocation[] {
    return this.locations.filter(
      (loc) => loc.country === country && loc.city === city,
    );
  }
}
