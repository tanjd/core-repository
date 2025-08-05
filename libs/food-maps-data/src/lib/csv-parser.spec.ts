import {
  generateLocationId,
  extractCityFromFilename,
  parseCsvContent,
  mergeFoodLocations,
} from "./csv-parser";
import { FoodLocation } from "./types";

describe("csv-parser", () => {
  describe("generateLocationId", () => {
    it("should generate consistent IDs for same input", () => {
      const id1 = generateLocationId("Sushi Place", "http://maps.google.com/1");
      const id2 = generateLocationId("Sushi Place", "http://maps.google.com/1");
      expect(id1).toBe(id2);
    });

    it("should generate different IDs for different inputs", () => {
      const id1 = generateLocationId("Sushi Place", "http://maps.google.com/1");
      const id2 = generateLocationId("Ramen Place", "http://maps.google.com/2");
      expect(id1).not.toBe(id2);
    });
  });

  describe("extractCityFromFilename", () => {
    it("should extract city name from filename", () => {
      expect(extractCityFromFilename("Tokyo (Food).csv")).toBe("Tokyo");
      expect(extractCityFromFilename("New York (Food).csv")).toBe("New York");
    });
  });

  describe("parseCsvContent", () => {
    const sampleCsv = `Title,Note,URL,Tags,Comment
Sushi Place,Great sushi,http://maps.google.com/1,Red,
Ramen Shop,Amazing ramen,http://maps.google.com/2,Yellow,`;

    it("should parse CSV content correctly", () => {
      const locations = parseCsvContent(sampleCsv, "Tokyo", "Japan");

      expect(locations).toHaveLength(2);
      expect(locations[0]).toMatchObject({
        name: "Sushi Place",
        description: "Great sushi",
        googleMapsUrl: "http://maps.google.com/1",
        tags: ["Red"],
        city: "Tokyo",
        country: "Japan",
      });
    });

    it("should skip empty rows", () => {
      const csvWithEmpty = `Title,Note,URL,Tags,Comment
Sushi Place,Great sushi,http://maps.google.com/1,Red,
,,,,`;

      const locations = parseCsvContent(csvWithEmpty, "Tokyo", "Japan");
      expect(locations).toHaveLength(1);
    });
  });

  describe("mergeFoodLocations", () => {
    const existingLocation: FoodLocation = {
      id: "123",
      name: "Sushi Place",
      description: "Great sushi",
      googleMapsUrl: "http://maps.google.com/1",
      tags: ["Red"],
      city: "Tokyo",
      country: "Japan",
      lastUpdated: new Date("2025-01-01"),
    };

    const newLocation: FoodLocation = {
      id: "456",
      name: "Ramen Shop",
      description: "Amazing ramen",
      googleMapsUrl: "http://maps.google.com/2",
      tags: ["Yellow"],
      city: "Tokyo",
      country: "Japan",
      lastUpdated: new Date("2025-01-02"),
    };

    it("should add new locations", () => {
      const result = mergeFoodLocations([existingLocation], [newLocation]);
      expect(result.added).toBe(1);
      expect(result.updated).toBe(0);
      expect(result.skipped).toBe(0);
    });

    it("should update changed locations", () => {
      const updatedLocation = {
        ...existingLocation,
        description: "Updated description",
      };
      const result = mergeFoodLocations([existingLocation], [updatedLocation]);
      expect(result.updated).toBe(1);
      expect(result.added).toBe(0);
      expect(result.skipped).toBe(0);
    });

    it("should skip unchanged locations", () => {
      const result = mergeFoodLocations([existingLocation], [existingLocation]);
      expect(result.skipped).toBe(1);
      expect(result.updated).toBe(0);
      expect(result.added).toBe(0);
    });
  });
});
