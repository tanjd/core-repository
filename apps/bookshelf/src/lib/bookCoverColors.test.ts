import { hashString, gradientForTitle, wrapLines } from "./bookCoverColors";

describe("hashString", () => {
  it("is deterministic", () => {
    expect(hashString("The Hobbit")).toBe(hashString("The Hobbit"));
  });

  it("is non-negative", () => {
    expect(hashString("")).toBeGreaterThanOrEqual(0);
    expect(hashString("Some Title")).toBeGreaterThanOrEqual(0);
  });

  it("differs for different strings", () => {
    expect(hashString("The Hobbit")).not.toBe(hashString("The Silmarillion"));
  });
});

describe("gradientForTitle", () => {
  it("is deterministic", () => {
    expect(gradientForTitle("Dune")).toEqual(gradientForTitle("Dune"));
  });

  it("produces valid hsl() strings with hue in range", () => {
    const { from, to } = gradientForTitle("Dune");

    for (const color of [from, to]) {
      const match = color.match(/^hsl\((\d+) \d+% \d+%\)$/);
      expect(match).not.toBeNull();
      const hue = Number(match?.[1]);
      expect(hue).toBeGreaterThanOrEqual(0);
      expect(hue).toBeLessThan(360);
    }
  });

  it("produces different colors for different titles", () => {
    expect(gradientForTitle("Dune")).not.toEqual(
      gradientForTitle("Neuromancer"),
    );
  });
});

describe("wrapLines", () => {
  it("returns an empty array for empty text", () => {
    expect(wrapLines("", 15, 3)).toEqual([]);
    expect(wrapLines("   ", 15, 3)).toEqual([]);
  });

  it("wraps text across multiple lines within the limit", () => {
    const lines = wrapLines("The Go Programming Language", 15, 3);

    expect(lines.length).toBeLessThanOrEqual(3);
    for (const line of lines) {
      expect(line.length).toBeLessThanOrEqual(15);
    }
    expect(lines.join(" ")).toBe("The Go Programming Language");
  });

  it("truncates with an ellipsis when content exceeds maxLines", () => {
    const lines = wrapLines(
      "A Very Long Title That Definitely Will Not Fit In Three Short Lines At All",
      10,
      3,
    );

    expect(lines.length).toBe(3);
    expect(lines[2].endsWith("…")).toBe(true);
  });

  it("does not add an ellipsis when everything fits", () => {
    const lines = wrapLines("Dune", 15, 3);

    expect(lines).toEqual(["Dune"]);
  });

  it("handles a single word longer than maxCharsPerLine", () => {
    const lines = wrapLines("Supercalifragilisticexpialidocious", 10, 1);

    expect(lines.length).toBe(1);
    expect(lines[0].length).toBeGreaterThan(0);
  });

  it("truncates a long word sharing a line with another word", () => {
    const lines = wrapLines("Supercalifragilisticexpialidocious Two", 10, 1);

    expect(lines.length).toBe(1);
    expect(lines[0].endsWith("…")).toBe(true);
  });
});
