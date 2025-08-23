-- Enable foreign keys and other good practices
PRAGMA foreign_keys = ON;
PRAGMA journal_mode = WAL;

-- Countries table
CREATE TABLE IF NOT EXISTS countries (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL UNIQUE
);

-- Cities table
CREATE TABLE IF NOT EXISTS cities (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL,
  country_id INTEGER NOT NULL,
  FOREIGN KEY (country_id) REFERENCES countries(id),
  UNIQUE(name, country_id)
);

-- Tags table for efficient storage
CREATE TABLE IF NOT EXISTS tags (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL UNIQUE
);

-- Locations table
CREATE TABLE IF NOT EXISTS locations (
  id TEXT PRIMARY KEY, -- UUID string
  name TEXT NOT NULL,
  description TEXT,
  google_maps_url TEXT NOT NULL,
  city_id INTEGER NOT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (city_id) REFERENCES cities(id)
);

-- Location tags junction table
CREATE TABLE IF NOT EXISTS location_tags (
  location_id TEXT NOT NULL,
  tag_id INTEGER NOT NULL,
  PRIMARY KEY (location_id, tag_id),
  FOREIGN KEY (location_id) REFERENCES locations(id) ON DELETE CASCADE,
  FOREIGN KEY (tag_id) REFERENCES tags(id)
);

-- Indices for better performance
CREATE INDEX IF NOT EXISTS idx_locations_city ON locations(city_id);
CREATE INDEX IF NOT EXISTS idx_cities_country ON cities(country_id);
CREATE INDEX IF NOT EXISTS idx_location_tags_tag ON location_tags(tag_id);

-- Trigger to update the updated_at timestamp
CREATE TRIGGER IF NOT EXISTS locations_updated_at
AFTER UPDATE ON locations
BEGIN
  UPDATE locations
  SET updated_at = CURRENT_TIMESTAMP
  WHERE id = NEW.id;
END;
