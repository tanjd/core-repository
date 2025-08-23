import { ApiClient } from "@tanjd/food-maps-data";
import Link from "next/link";

interface PageProps {
  params: {
    country: string;
    city: string;
  };
}

async function getLocations(country: string, city: string) {
  try {
    const apiClient = new ApiClient();

    // Check if backend is available
    const isHealthy = await apiClient.healthCheck();
    if (!isHealthy) {
      console.warn("Backend is not available, returning empty data");
      return [];
    }

    return await apiClient.getLocationsByCity(
      decodeURIComponent(country),
      decodeURIComponent(city),
    );
  } catch (error) {
    console.error("Error loading locations:", error);
    return [];
  }
}

export default async function CityPage({ params }: PageProps) {
  const { country, city } = params;
  const locations = await getLocations(country, city);
  const decodedCity = decodeURIComponent(city);
  const decodedCountry = decodeURIComponent(country);

  return (
    <main className="container mx-auto px-4 py-8">
      <div className="mb-8 flex items-center gap-4">
        <Link
          href="/"
          className="text-blue-600 hover:text-blue-800 flex items-center gap-1"
        >
          <svg
            xmlns="http://www.w3.org/2000/svg"
            width="20"
            height="20"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            strokeWidth="2"
            strokeLinecap="round"
            strokeLinejoin="round"
          >
            <path d="M19 12H5M12 19l-7-7 7-7" />
          </svg>
          Back to Countries
        </Link>
        <h1 className="text-4xl font-bold">
          {decodedCity}, {decodedCountry}
        </h1>
      </div>

      {locations.length === 0 ? (
        <div className="bg-yellow-50 border border-yellow-200 rounded-lg p-6">
          <h2 className="text-lg font-medium text-yellow-800 mb-2">
            No locations found
          </h2>
          <p className="text-yellow-700">
            No food locations found for {decodedCity}, {decodedCountry}.
          </p>
        </div>
      ) : (
        <div className="grid gap-6 md:grid-cols-2 lg:grid-cols-3">
          {locations.map((location) => (
            <div
              key={location.id}
              className="bg-white rounded-lg shadow-md overflow-hidden hover:shadow-lg transition-shadow"
            >
              <div className="p-6">
                <h2 className="text-xl font-semibold mb-2">
                  <a
                    href={location.googleMapsUrl}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="text-blue-600 hover:text-blue-800"
                  >
                    {location.name}
                  </a>
                </h2>

                {location.description && (
                  <p className="text-gray-600 mb-4">{location.description}</p>
                )}

                {location.tags.length > 0 && (
                  <div className="flex flex-wrap gap-2">
                    {location.tags.map((tag) => (
                      <span
                        key={tag}
                        className="px-2 py-1 bg-gray-100 text-gray-700 text-sm rounded-full"
                      >
                        {tag}
                      </span>
                    ))}
                  </div>
                )}
              </div>
            </div>
          ))}
        </div>
      )}
    </main>
  );
}
