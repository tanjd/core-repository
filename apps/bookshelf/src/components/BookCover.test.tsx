import { fireEvent, render } from "@testing-library/react";
import { BookCover } from "./BookCover";

// Substituting a plain <img> for next/image is the whole point of the
// mock — jsdom has no image optimizer to run, and asserting on
// container.querySelector("img") is how these tests distinguish the
// real cover from the SVG fallback. next/image's lint rule doesn't
// apply here.
jest.mock("next/image", () => ({
  __esModule: true,
  default: (props: Record<string, unknown>) => {
    const imgProps = { ...props };
    delete imgProps.fill;

    // eslint-disable-next-line @next/next/no-img-element
    return <img {...imgProps} alt={props.alt as string} />;
  },
}));

describe("BookCover", () => {
  it("renders the cover image when coverUrl is present", () => {
    const { container } = render(
      <BookCover
        title="Dune"
        coverUrl="https://example.com/dune.jpg"
        sizes="100px"
      />,
    );

    const img = container.querySelector("img");
    expect(img).not.toBeNull();
    expect(img).toHaveAttribute("src", "https://example.com/dune.jpg");
    expect(container.querySelector("svg")).toBeNull();
  });

  it("renders a generated SVG fallback when coverUrl is empty", () => {
    const { container } = render(
      <BookCover title="Dune" author="Frank Herbert" sizes="100px" />,
    );

    expect(container.querySelector("img")).toBeNull();
    expect(container.querySelector("svg")).not.toBeNull();
  });

  it("includes the title and author text in the fallback", () => {
    const { getByText } = render(
      <BookCover title="Dune" author="Frank Herbert" sizes="100px" />,
    );

    expect(getByText("Dune")).toBeInTheDocument();
    expect(getByText("Frank Herbert")).toBeInTheDocument();
  });

  it("renders no author text when author is missing", () => {
    const { container } = render(<BookCover title="Dune" sizes="100px" />);

    const texts = Array.from(container.querySelectorAll("text")).map(
      (el) => el.textContent,
    );
    expect(texts).toEqual(["Dune"]);
  });

  it("renders the generated SVG fallback if the cover image fails to load", () => {
    const { container } = render(
      <BookCover
        title="Dune"
        author="Frank Herbert"
        coverUrl="https://example.com/broken.jpg"
        sizes="100px"
      />,
    );

    const img = container.querySelector("img");
    expect(img).not.toBeNull();
    fireEvent.error(img as HTMLImageElement);

    expect(container.querySelector("img")).toBeNull();
    expect(container.querySelector("svg")).not.toBeNull();
  });

  it("applies object-cover by default so grid tiles fill their fixed 2:3 slot", () => {
    const { container } = render(
      <BookCover
        title="Dune"
        coverUrl="https://example.com/dune.jpg"
        sizes="100px"
      />,
    );

    const img = container.querySelector("img");
    expect(img?.className).toContain("object-cover");
    expect(img?.className).not.toContain("object-contain");
  });

  it('applies object-contain when fit="contain" so the whole cover shows unclipped', () => {
    // The book detail page uses this variant — real covers aren't all 2:3,
    // and cropping the hero image made different books look inconsistent.
    const { container } = render(
      <BookCover
        title="Dune"
        coverUrl="https://example.com/dune.jpg"
        sizes="100px"
        fit="contain"
      />,
    );

    const img = container.querySelector("img");
    expect(img?.className).toContain("object-contain");
    expect(img?.className).not.toContain("object-cover");
  });

  it("produces the same generated cover for the same title across renders", () => {
    const first = render(<BookCover title="Dune" sizes="100px" />);
    const second = render(<BookCover title="Dune" sizes="100px" />);

    const firstGradientId = first.container.querySelector("linearGradient")?.id;
    const secondGradientId =
      second.container.querySelector("linearGradient")?.id;

    expect(firstGradientId).toBeDefined();
    expect(firstGradientId).toBe(secondGradientId);
  });
});
