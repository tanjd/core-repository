import { render, screen } from "@testing-library/react";
import { PasswordMatchIndicator } from "./PasswordMatchIndicator";

describe("PasswordMatchIndicator", () => {
  it("renders nothing when confirm is empty", () => {
    const { container } = render(
      <PasswordMatchIndicator password="Passw0rd1234" confirm="" />,
    );

    expect(container).toBeEmptyDOMElement();
  });

  it("shows 'Matches' in text-success when the passwords match", () => {
    render(
      <PasswordMatchIndicator password="Passw0rd1234" confirm="Passw0rd1234" />,
    );

    const text = screen.getByText("Matches");
    expect(text).toBeInTheDocument();
    expect(text.closest("p")).toHaveClass("text-success");
  });

  it("shows 'Doesn't match yet' in text-muted-foreground when they differ", () => {
    render(
      <PasswordMatchIndicator password="Passw0rd1234" confirm="Passw0rd" />,
    );

    const text = screen.getByText("Doesn't match yet");
    expect(text).toBeInTheDocument();
    expect(text.closest("p")).toHaveClass("text-muted-foreground");
  });
});
