import { createHash } from "crypto";
import { parse } from "csv-parse/sync";
import { FoodLocation, CsvUploadResult } from "./types";

export function generateLocationId(name: string, url: string): string {
  return createHash("md5").update(`${name}-${url}`).digest("hex");
}

export function extractCityFromFilename(filename: string): string {
  // Example: "Tokyo (Food).csv" -> "Tokyo"
  return filename.replace(/\s*\(Food\)\.csv$/, "");
}

interface CsvRecord {
  Title: string;
  Note?: string;
  URL: string;
  Tags?: string;
  Comment?: string;
}

export function parseCsvContent(
  content: string,
  city: string,
  country: string,
): FoodLocation[] {
  // Find the header line
  const lines = content.split("\n");
  const headerIndex = lines.findIndex((line) =>
    line.startsWith("Title,Note,URL"),
  );
  if (headerIndex === -1) {
    throw new Error("CSV header not found");
  }

  // Parse only from the header line
  const csvContent = lines.slice(headerIndex).join("\n");
  const records = parse(csvContent, {
    columns: true,
    skip_empty_lines: true,
    trim: true,
  }) as CsvRecord[];

  return records
    .filter((record) => record.Title && record.URL) // Skip empty rows
    .map((record) => ({
      id: generateLocationId(record.Title, record.URL),
      name: record.Title,
      description: record.Note || "",
      googleMapsUrl: record.URL,
      tags: record.Tags ? record.Tags.split(",").map((t) => t.trim()) : [],
      city,
      country,
      lastUpdated: new Date(),
    }));
}

export function mergeFoodLocations(
  existing: FoodLocation[],
  newLocations: FoodLocation[],
): CsvUploadResult {
  const result: CsvUploadResult = {
    added: 0,
    updated: 0,
    skipped: 0,
    errors: [],
  };

  const existingById = new Map(existing.map((loc) => [loc.id, loc]));

  for (const newLoc of newLocations) {
    try {
      if (existingById.has(newLoc.id)) {
        const existingLoc = existingById.get(newLoc.id);
        if (!existingLoc) continue;

        // Only update if something changed
        if (
          existingLoc.name !== newLoc.name ||
          existingLoc.description !== newLoc.description ||
          existingLoc.googleMapsUrl !== newLoc.googleMapsUrl ||
          JSON.stringify(existingLoc.tags) !== JSON.stringify(newLoc.tags)
        ) {
          existingById.set(newLoc.id, {
            ...newLoc,
            lastUpdated: new Date(),
          });
          result.updated++;
        } else {
          result.skipped++;
        }
      } else {
        existingById.set(newLoc.id, newLoc);
        result.added++;
      }
    } catch (err) {
      const error = err as Error;
      result.errors.push(`Error processing ${newLoc.name}: ${error.message}`);
    }
  }

  return result;
}
