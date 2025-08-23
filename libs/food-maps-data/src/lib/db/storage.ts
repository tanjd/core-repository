import initSqlJs from "sql.js";
import { readFileSync, writeFileSync } from "fs";
import { join } from "path";
import { FoodLocation, CsvUploadResult, LocationGroup } from "../types";

export class StorageManager {
  private db: initSqlJs.Database | null = null;
  private dbPath: string;

  constructor(dbPath: string) {
    this.dbPath = dbPath;
  }

  async init(): Promise<void> {
    const SQL = await initSqlJs();

    try {
      // Try to load existing database
      const data = readFileSync(this.dbPath);
      this.db = new SQL.Database(data);
    } catch {
      // Create new database if file doesn't exist
      this.db = new SQL.Database();
      // Initialize schema
      const schema = readFileSync(join(__dirname, "schema.sql"), "utf-8");
      this.db.run(schema);
    }
  }

  addLocations(locations: FoodLocation[]): CsvUploadResult {
    if (!this.db) throw new Error("Database not initialized");

    const result: CsvUploadResult = {
      added: 0,
      updated: 0,
      skipped: 0,
      errors: [],
    };

    try {
      this.db.run("BEGIN TRANSACTION");

      for (const location of locations) {
        // Ensure country exists
        const countryStmt = this.db.prepare(
          "INSERT OR IGNORE INTO countries (name) VALUES (?)",
        );
        countryStmt.run([location.country]);
        countryStmt.free();

        const getCountryStmt = this.db.prepare(
          "SELECT id FROM countries WHERE name = ?",
        );
        const countryRow = getCountryStmt.get([location.country]);
        const countryId = countryRow ? (countryRow[0] as number) : undefined;
        getCountryStmt.free();

        if (!countryId) {
          throw new Error(
            `Failed to create or find country: ${location.country}`,
          );
        }

        // Ensure city exists
        const cityStmt = this.db.prepare(
          "INSERT OR IGNORE INTO cities (name, country_id) VALUES (?, ?)",
        );
        cityStmt.run([location.city, countryId]);
        cityStmt.free();

        const getCityStmt = this.db.prepare(
          "SELECT id FROM cities WHERE name = ? AND country_id = ?",
        );
        const cityRow = getCityStmt.get([location.city, countryId]);
        const cityId = cityRow ? (cityRow[0] as number) : undefined;
        getCityStmt.free();

        if (!cityId) {
          throw new Error(`Failed to create or find city: ${location.city}`);
        }

        // Check if location exists
        const checkLocStmt = this.db.prepare(
          "SELECT id FROM locations WHERE id = ?",
        );
        const existing = checkLocStmt.get([location.id]);
        checkLocStmt.free();

        if (existing) {
          // Update existing location
          const updateStmt = this.db.prepare(`
            UPDATE locations
            SET name = ?, description = ?, google_maps_url = ?, city_id = ?
            WHERE id = ?`);
          updateStmt.run([
            location.name,
            location.description,
            location.googleMapsUrl,
            cityId,
            location.id,
          ]);
          updateStmt.free();
          result.updated++;
        } else {
          // Insert new location
          const insertStmt = this.db.prepare(`
            INSERT INTO locations
            (id, name, description, google_maps_url, city_id)
            VALUES (?, ?, ?, ?, ?)`);
          insertStmt.run([
            location.id,
            location.name,
            location.description,
            location.googleMapsUrl,
            cityId,
          ]);
          insertStmt.free();
          result.added++;
        }

        // Handle tags
        const deleteTagsStmt = this.db.prepare(
          "DELETE FROM location_tags WHERE location_id = ?",
        );
        deleteTagsStmt.run([location.id]);
        deleteTagsStmt.free();

        for (const tag of location.tags) {
          // Insert tag if it doesn't exist
          const insertTagStmt = this.db.prepare(
            "INSERT OR IGNORE INTO tags (name) VALUES (?)",
          );
          insertTagStmt.run([tag]);
          insertTagStmt.free();

          // Get tag ID
          const getTagStmt = this.db.prepare(
            "SELECT id FROM tags WHERE name = ?",
          );
          const tagRow = getTagStmt.get([tag]);
          const tagId = tagRow ? (tagRow[0] as number) : undefined;
          getTagStmt.free();

          if (!tagId) {
            throw new Error(`Failed to create or find tag: ${tag}`);
          }

          // Link tag to location
          const linkTagStmt = this.db.prepare(
            "INSERT INTO location_tags (location_id, tag_id) VALUES (?, ?)",
          );
          linkTagStmt.run([location.id, tagId]);
          linkTagStmt.free();
        }
      }

      this.db.run("COMMIT");

      // Save changes to file
      const data = this.db.export();
      writeFileSync(this.dbPath, Buffer.from(data));
    } catch (error) {
      this.db.run("ROLLBACK");
      result.errors.push(
        error instanceof Error ? error.message : String(error),
      );
    }

    return result;
  }

  getLocationsByCountry(): LocationGroup[] {
    if (!this.db) throw new Error("Database not initialized");

    const stmt = this.db.prepare(`
      SELECT
        c.name as country,
        ci.name as city,
        COUNT(l.id) as location_count
      FROM countries c
      JOIN cities ci ON ci.country_id = c.id
      LEFT JOIN locations l ON l.city_id = ci.id
      GROUP BY c.name, ci.name
      ORDER BY c.name, ci.name
    `);

    const rows: Array<{
      country: string;
      city: string;
      location_count: number;
    }> = [];
    while (stmt.step()) {
      const obj = stmt.getAsObject();
      rows.push({
        country: String(obj["country"]),
        city: String(obj["city"]),
        location_count: Number(obj["location_count"]),
      });
    }
    stmt.free();

    const groups: LocationGroup[] = [];
    let currentGroup: LocationGroup | null = null;

    for (const row of rows) {
      if (!currentGroup || currentGroup.country !== row.country) {
        currentGroup = {
          country: row.country,
          cities: [],
          totalLocations: 0,
        };
        groups.push(currentGroup);
      }

      currentGroup.cities.push({
        name: row.city,
        locationCount: row.location_count,
      });
      currentGroup.totalLocations += row.location_count;
    }

    return groups;
  }

  getLocationsByCity(countryName: string, cityName: string): FoodLocation[] {
    if (!this.db) throw new Error("Database not initialized");

    const stmt = this.db.prepare(`
      SELECT
        l.id,
        l.name,
        l.description,
        l.google_maps_url as googleMapsUrl,
        ci.name as city,
        c.name as country,
        l.updated_at as lastUpdated,
        GROUP_CONCAT(t.name) as tags
      FROM locations l
      JOIN cities ci ON ci.id = l.city_id
      JOIN countries c ON c.id = ci.country_id
      LEFT JOIN location_tags lt ON lt.location_id = l.id
      LEFT JOIN tags t ON t.id = lt.tag_id
      WHERE c.name = ? AND ci.name = ?
      GROUP BY l.id
      ORDER BY l.name
    `);

    const locations: FoodLocation[] = [];
    stmt.bind([countryName, cityName]);

    while (stmt.step()) {
      const obj = stmt.getAsObject();
      locations.push({
        id: String(obj["id"]),
        name: String(obj["name"]),
        description: String(obj["description"]),
        googleMapsUrl: String(obj["googleMapsUrl"]),
        city: String(obj["city"]),
        country: String(obj["country"]),
        lastUpdated: new Date(String(obj["lastUpdated"])),
        tags: obj["tags"] ? String(obj["tags"]).split(",") : [],
      });
    }
    stmt.free();

    return locations;
  }

  close(): void {
    if (this.db) {
      const data = this.db.export();
      writeFileSync(this.dbPath, Buffer.from(data));
      this.db.close();
      this.db = null;
    }
  }
}
