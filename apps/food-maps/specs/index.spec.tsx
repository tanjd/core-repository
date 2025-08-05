import { render } from "@testing-library/react";
import HomePage from "../src/app/page";

jest.mock("@tanjd/food-maps-data", () => ({
  StorageManager: jest.fn().mockImplementation(() => ({
    load: jest.fn(),
    getLocationsByCountry: jest.fn().mockReturnValue([
      {
        country: "Test Country",
        cities: [{ name: "Test City", locations: [] }],
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
