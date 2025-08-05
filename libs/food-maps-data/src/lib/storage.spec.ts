import { StorageManager } from "./storage";
import { FoodLocation } from "./types";
import { writeFile, unlink } from "fs/promises";
import { join } from "path";
import { tmpdir } from "os";

describe("StorageManager", () => {
  let storageManager: StorageManager;
  let testFilePath: string;

  const sampleLocation: FoodLocation = {
    id: "123",
    name: "Sushi Place",
    description: "Great sushi",
    googleMapsUrl: "http://maps.google.com/1",
    tags: ["Red"],
    city: "Tokyo",
    country: "Japan",
    lastUpdated: new Date("2025-01-01"),
  };

  beforeEach(async () => {
    testFilePath = join(tmpdir(), `test-${Date.now()}.json`);
    storageManager = new StorageManager(testFilePath);
  });

  afterEach(async () => {
    try {
      await unlink(testFilePath);
    } catch (error) {
      // Ignore if file doesn't exist
    }
  });

  it("should save and load locations", async () => {
    await storageManager.addLocations([sampleLocation]);

    // Create new instance to test loading
    const newManager = new StorageManager(testFilePath);
    await newManager.load();

    const locations = newManager.getLocationsByCity("Japan", "Tokyo");
    expect(locations).toHaveLength(1);
    expect(locations[0]).toMatchObject({
      name: "Sushi Place",
      city: "Tokyo",
      country: "Japan",
    });
  });

  it("should group locations by country", async () => {
    const tokyoLocation = sampleLocation;
    const osakaLocation: FoodLocation = {
      ...sampleLocation,
      id: "456",
      city: "Osaka",
    };
    const seoulLocation: FoodLocation = {
      ...sampleLocation,
      id: "789",
      city: "Seoul",
      country: "South Korea",
    };

    await storageManager.addLocations([
      tokyoLocation,
      osakaLocation,
      seoulLocation,
    ]);

    const groups = storageManager.getLocationsByCountry();
    expect(groups).toHaveLength(2); // Japan and South Korea

    const japan = groups.find((g) => g.country === "Japan");
    expect(japan?.cities).toHaveLength(2); // Tokyo and Osaka
    expect(japan?.totalLocations).toBe(2);

    const korea = groups.find((g) => g.country === "South Korea");
    expect(korea?.cities).toHaveLength(1); // Seoul
    expect(korea?.totalLocations).toBe(1);
  });

  it("should handle empty storage file", async () => {
    await storageManager.load(); // No file exists yet
    const locations = storageManager.getLocationsByCountry();
    expect(locations).toHaveLength(0);
  });
});
