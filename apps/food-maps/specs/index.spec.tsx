import { render, screen } from "@testing-library/react";
import HomePage from "../src/app/page";

// Mock ApiClient
jest.mock("@tanjd/food-maps-data", () => ({
  ApiClient: jest.fn().mockImplementation(() => ({
    healthCheck: jest.fn().mockResolvedValue(true),
    getLocationsByCountry: jest.fn().mockResolvedValue([
      {
        country: "Test Country",
        cities: [
          {
            name: "Test City",
            locationCount: 0,
          },
        ],
        totalLocations: 0,
      },
    ]),
  })),
}));

describe("HomePage", () => {
  it("should render successfully", async () => {
    const { baseElement } = render(await HomePage());
    expect(baseElement).toBeTruthy();
  });
});
