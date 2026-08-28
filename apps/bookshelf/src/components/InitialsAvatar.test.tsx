import { render } from "@testing-library/react";
import { InitialsAvatar } from "./InitialsAvatar";

describe("InitialsAvatar", () => {
  it("renders the first letter of up to two words, uppercased", () => {
    const { container } = render(<InitialsAvatar name="ada lovelace" />);
    expect(container).toHaveTextContent("AL");
  });

  it("falls back to '?' for an empty name", () => {
    const { container } = render(<InitialsAvatar name="" />);
    expect(container).toHaveTextContent("?");
  });
});
