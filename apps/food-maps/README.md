# Food Maps

A web application for managing and viewing your saved food locations from Google Maps.

## Features

- View food locations grouped by country and city
- Import CSV files exported from Google Maps
- Automatic deduplication of locations
- Tags and descriptions support
- Direct links to Google Maps

## Getting Started

1. Clone the repository
2. Install dependencies: `pnpm install`
3. Start the development server: `pnpm nx serve food-maps`

The app will be available at http://localhost:3000.

## Data Structure

The app uses a master JSON file to store all locations. This file is automatically created when you first import data.

### Location Data Model

```typescript
interface FoodLocation {
  id: string; // Unique identifier
  name: string; // Place name
  description: string; // Notes/description
  googleMapsUrl: string; // Google Maps link
  tags: string[]; // Categories/labels
  city: string; // City name
  country: string; // Country name
  lastUpdated: Date; // Last modification date
}
```

## Importing Data

### CSV File Requirements

1. Files must be named in the format: `City Name (Food).csv`

   - Example: `Tokyo (Food).csv`, `London (Food).csv`
   - The city name must match one of the supported cities in `scripts/city-country-map.ts`

2. CSV Structure:

   ```csv
   Title,Note,URL,Tags,Comment
   "Restaurant Name","Description","https://maps.google.com/...","tag1,tag2",""
   ```

   - Title: Name of the location
   - Note: Description or notes (optional)
   - URL: Google Maps URL
   - Tags: Comma-separated list of tags (optional)
   - Comment: Additional comments (optional)

### Import Process

1. Place your CSV files in the `/data/food-maps/` directory
2. Visit `/upload` in the app to process the files
3. The app will:
   - Parse each CSV file
   - Extract city/country information from filenames
   - Generate unique IDs to prevent duplicates
   - Merge new locations with existing ones
   - Show import results including any errors

### Handling Import Errors

Common issues and solutions:

1. **"Invalid Record Length" Error**

   - Cause: CSV file has inconsistent column counts
   - Fix: Ensure all rows have the same number of columns
   - Common issues:
     - Extra commas in fields (use quotes)
     - Missing fields (add empty quotes)
     - Line breaks in fields (use quotes)

2. **"Unknown City" Warning**

   - Cause: City name not found in `city-country-map.ts`
   - Fix: Add the city-country mapping to `scripts/city-country-map.ts`

3. **"Failed to parse CSV" Error**
   - Cause: Malformed CSV content
   - Fix:
     - Check for proper CSV formatting
     - Ensure UTF-8 encoding
     - Remove any BOM markers

## Project Structure

```
food-maps/
├── src/
│   ├── app/
│   │   ├── page.tsx              # Homepage (country/city list)
│   │   ├── upload/
│   │   │   └── page.tsx          # CSV upload page
│   │   └── locations/
│   │       └── [country]/
│   │           └── [city]/
│   │               └── page.tsx   # City details page
│   └── scripts/
│       ├── init-data.ts          # Data initialization script
│       └── city-country-map.ts   # City to country mappings
```

## Adding New Cities

To add support for a new city:

1. Open `scripts/city-country-map.ts`
2. Add your city-country mapping:
   ```typescript
   export const cityToCountry: Record<string, string> = {
     // ... existing mappings ...
     "New City": "Country Name",
   };
   ```
3. Ensure your CSV filename matches the city name exactly

## Development

- Built with Next.js 15
- Uses Nx for monorepo management
- Tailwind CSS for styling
- TypeScript for type safety

### Available Scripts

- `pnpm nx serve food-maps` - Start development server
- `pnpm nx build food-maps` - Build for production
- `pnpm nx lint food-maps` - Run linting
- `pnpm nx test food-maps` - Run tests
