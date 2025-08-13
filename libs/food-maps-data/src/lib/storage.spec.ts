import { StorageManager } from "./storage";
import { FoodLocation } from "./types";

// Mock sql.js
jest.mock("sql.js", () => ({
  __esModule: true,
  default: jest.fn().mockResolvedValue({
    Database: jest.fn().mockImplementation(() => ({
      exec: jest.fn(),
      run: jest.fn(),
      prepare: jest.fn().mockReturnValue({
        run: jest.fn(),
        get: jest.fn(),
        getAsObject: jest.fn(),
        free: jest.fn(),
      }),
      close: jest.fn(),
    })),
  }),
}));

// Mock fs
jest.mock("fs", () => ({
  readFileSync: jest.fn(),
  writeFileSync: jest.fn(),
  existsSync: jest.fn(),
}));

describe("StorageManager", () => {
  let storage: StorageManager;

  beforeEach(() => {
    storage = new StorageManager("/test/db.sqlite");
  });

  it("should initialize successfully", async () => {
    await storage.init();
    expect(storage).toBeDefined();
  });

  it("should add locations", async () => {
    await storage.init();
    const locations: FoodLocation[] = [
      {
        id: "1",
        title: "Test Location",
        note: "Test Note",
        url: "http://test.com",
        city: "Test City",
        country: "Test Country",
      },
    ];
    await storage.addLocations(locations);
  });
});
