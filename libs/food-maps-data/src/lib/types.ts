export interface FoodLocation {
  id: string;
  name: string;
  description: string;
  googleMapsUrl: string;
  tags: string[];
  city: string;
  country: string;
  lastUpdated: Date;
}

export interface CsvUploadResult {
  added: number;
  updated: number;
  skipped: number;
  errors: string[];
}

export interface CityInfo {
  name: string;
  locationCount: number;
}

export interface LocationGroup {
  country: string;
  cities: CityInfo[];
  totalLocations: number;
}
