export interface FoodLocation {
  id: string; // Generated from name + url
  name: string;
  description: string;
  googleMapsUrl: string;
  tags: string[];
  city: string;
  country: string;
  lastUpdated: Date;
}

export interface CsvUploadResult {
  added: number; // New locations added
  updated: number; // Existing locations updated
  skipped: number; // Duplicates skipped
  errors: string[]; // Any parsing errors
}

export interface LocationGroup {
  country: string;
  cities: {
    name: string;
    locations: FoodLocation[];
  }[];
  totalLocations: number;
}
