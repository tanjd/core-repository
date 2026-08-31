import { parsePhone } from "./parsePhone";

describe("parsePhone", () => {
  it("defaults to Singapore for empty/undefined input", () => {
    expect(parsePhone(undefined)).toEqual({ iso2: "SG", localNumber: "" });
    expect(parsePhone("")).toEqual({ iso2: "SG", localNumber: "" });
    expect(parsePhone(null)).toEqual({ iso2: "SG", localNumber: "" });
  });

  it("defaults to Singapore for a bare local number with no country code", () => {
    expect(parsePhone("9123 4567")).toEqual({
      iso2: "SG",
      localNumber: "9123 4567",
    });
  });

  it("parses a Singapore number", () => {
    expect(parsePhone("+65 9123 4567")).toEqual({
      iso2: "SG",
      localNumber: "9123 4567",
    });
  });

  it("matches a multi-digit dial code ahead of any shorter false prefix", () => {
    expect(parsePhone("+852 1234 5678")).toEqual({
      iso2: "HK",
      localNumber: "1234 5678",
    });
  });

  it("falls back to Singapore for an unrecognized dial code", () => {
    expect(parsePhone("+999 123")).toEqual({
      iso2: "SG",
      localNumber: "+999 123",
    });
  });
});
