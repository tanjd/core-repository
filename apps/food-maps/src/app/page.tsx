import { StorageManager } from "@tanjd/food-maps-data";
import { join } from "path";
import Link from "next/link";

async function getLocations() {
  try {
    const filePath = join(process.cwd(), "../../data/master.json");
    console.log("Loading data from:", filePath);

    const storage = new StorageManager(filePath);
    await storage.load();

    const groups = storage.getLocationsByCountry();
    console.log("Loaded groups:", groups);

    return groups;
  } catch (error) {
    console.error("Error loading locations:", error);
    return [];
  }
}

export default async function HomePage() {
  const locationGroups = await getLocations();

  if (!locationGroups || locationGroups.length === 0) {
    return (
      <main className="container mx-auto px-4 py-8">
        <h1 className="text-4xl font-bold mb-8">My Food Maps</h1>
        <div className="bg-yellow-50 border border-yellow-200 rounded-lg p-6">
          <h2 className="text-lg font-medium text-yellow-800 mb-2">
            No locations found
          </h2>
          <p className="text-yellow-700">
            Make sure the data file exists at /workspace/data/master.json and
            contains valid location data.
          </p>
        </div>
      </main>
    );
  }

  return (
    <main className="container mx-auto px-4 py-8">
      <h1 className="text-4xl font-bold mb-8">My Food Maps</h1>

      <div className="grid gap-6 md:grid-cols-2 lg:grid-cols-3">
        {locationGroups.map((group) => (
          <div
            key={group.country}
            className="bg-white rounded-lg shadow-md overflow-hidden hover:shadow-lg transition-shadow"
          >
            <div className="p-6">
              <h2 className="text-2xl font-semibold mb-2">{group.country}</h2>
              <p className="text-gray-600 mb-4">
                {group.totalLocations} locations across {group.cities.length}{" "}
                cities
              </p>

              <div className="space-y-2">
                {group.cities.map((city) => (
                  <Link
                    key={city.name}
                    href={`/locations/${encodeURIComponent(group.country)}/${encodeURIComponent(city.name)}`}
                    className="block p-3 bg-gray-50 rounded hover:bg-gray-100 transition-colors"
                  >
                    <span className="font-medium">{city.name}</span>
                    <span className="text-gray-500 ml-2">
                      ({city.locations.length} spots)
                    </span>
                  </Link>
                ))}
              </div>
            </div>
          </div>
        ))}
      </div>
    </main>
  );
}
