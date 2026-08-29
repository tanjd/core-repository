import { render, screen } from "@testing-library/react";
import { PasswordStrengthMeter } from "./PasswordStrengthMeter";

describe("PasswordStrengthMeter", () => {
  it("renders nothing when password is empty", () => {
    const { container } = render(<PasswordStrengthMeter password="" />);

    expect(container).toBeEmptyDOMElement();
  });

  it("shows a red X for length when the password is too short", () => {
    render(<PasswordStrengthMeter password="Ab1" />);

    const item = screen.getByText("At least 12 characters");
    expect(item.closest("li")).toHaveClass("text-muted-foreground");
  });

  it("shows a green check for length once the password is long enough", () => {
    render(<PasswordStrengthMeter password="Str0ngPassw0rd!" />);

    const item = screen.getByText("At least 12 characters");
    expect(item.closest("li")).toHaveClass("text-success");
  });

  it("shows a red X for case variety when only lowercase letters are used", () => {
    render(<PasswordStrengthMeter password="alllowercase1" />);

    const item = screen.getByText("Uppercase and lowercase letters");
    expect(item.closest("li")).toHaveClass("text-muted-foreground");
  });

  it("shows a red X for the numeral requirement when there is no digit", () => {
    render(<PasswordStrengthMeter password="NoDigitsHere" />);

    const item = screen.getByText("At least one number");
    expect(item.closest("li")).toHaveClass("text-muted-foreground");
  });

  it("shows a red X when the password contains a disallowed name or email", () => {
    render(
      <PasswordStrengthMeter
        password="MyNameIsAda123"
        disallowed={["Ada", "ada"]}
      />,
    );

    const item = screen.getByText("Doesn't contain your name or email");
    expect(item.closest("li")).toHaveClass("text-muted-foreground");
  });

  it("shows a red X for a commonly used password", () => {
    render(<PasswordStrengthMeter password="password" />);

    const item = screen.getByText("Not a commonly used password");
    expect(item.closest("li")).toHaveClass("text-muted-foreground");
  });

  it("shows a green check when the password is not a common one", () => {
    render(<PasswordStrengthMeter password="Str0ngPassw0rd!" />);

    const item = screen.getByText("Not a commonly used password");
    expect(item.closest("li")).toHaveClass("text-success");
  });
});
