export interface FoodLocation {
  id: string;
  name: string;
  description: string;
  googleMapsUrl: string;
  city: string;
  country: string;
  lastUpdated: Date;
  tags: string[];
}

export interface LocationGroup {
  country: string;
  cities: Array<{
    name: string;
    locationCount: number;
  }>;
  totalLocations: number;
}

export interface CsvUploadResult {
  added: number;
  updated: number;
  skipped: number;
  errors: string[];
}

export class ApiClient {
  private baseUrl: string;

  constructor(baseUrl = "http://localhost:8080") {
    this.baseUrl = baseUrl;
  }

  async getLocationsByCountry(): Promise<LocationGroup[]> {
    try {
      const response = await fetch(`${this.baseUrl}/api/locations/by-country`);
      if (!response.ok) {
        throw new Error(`HTTP error! status: ${response.status}`);
      }
      return await response.json();
    } catch (error) {
      console.error("Error fetching locations by country:", error);
      return [];
    }
  }

  async getLocationsByCity(
    countryName: string,
    cityName: string,
  ): Promise<FoodLocation[]> {
    try {
      const response = await fetch(
        `${this.baseUrl}/api/locations/by-city?country=${encodeURIComponent(countryName)}&city=${encodeURIComponent(cityName)}`,
      );
      if (!response.ok) {
        throw new Error(`HTTP error! status: ${response.status}`);
      }
      return await response.json();
    } catch (error) {
      console.error("Error fetching locations by city:", error);
      return [];
    }
  }

  async addLocations(locations: FoodLocation[]): Promise<CsvUploadResult> {
    try {
      const response = await fetch(`${this.baseUrl}/api/locations`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify(locations),
      });

      if (!response.ok) {
        throw new Error(`HTTP error! status: ${response.status}`);
      }

      return await response.json();
    } catch (error) {
      console.error("Error adding locations:", error);
      return {
        added: 0,
        updated: 0,
        skipped: 0,
        errors: [error instanceof Error ? error.message : String(error)],
      };
    }
  }

  async healthCheck(): Promise<boolean> {
    try {
      const response = await fetch(`${this.baseUrl}/health`);
      return response.ok;
    } catch (error) {
      console.error("Health check failed:", error);
      return false;
    }
  }
}
