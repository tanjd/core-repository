import { render } from "@testing-library/react";
import { BookCover } from "./BookCover";

jest.mock("next/image", () => ({
  __esModule: true,
  default: (props: Record<string, unknown>) => {
    const imgProps = { ...props };
    delete imgProps.fill;

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
