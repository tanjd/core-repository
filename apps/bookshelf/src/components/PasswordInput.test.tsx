import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { PasswordInput } from "./PasswordInput";

describe("PasswordInput", () => {
  it("renders an input with type=password by default", () => {
    render(<PasswordInput placeholder="Password" />);

    expect(screen.getByPlaceholderText("Password")).toHaveAttribute(
      "type",
      "password",
    );
  });

  it("shows a 'Show password' toggle button when masked", () => {
    render(<PasswordInput placeholder="Password" />);

    expect(
      screen.getByRole("button", { name: "Show password" }),
    ).toBeInTheDocument();
  });

  it("switches the input to type=text when the toggle is clicked", async () => {
    const user = userEvent.setup();
    render(<PasswordInput placeholder="Password" />);

    await user.click(screen.getByRole("button", { name: "Show password" }));

    expect(screen.getByPlaceholderText("Password")).toHaveAttribute(
      "type",
      "text",
    );
    expect(
      screen.getByRole("button", { name: "Hide password" }),
    ).toBeInTheDocument();
  });

  it("switches back to type=password when clicked again", async () => {
    const user = userEvent.setup();
    render(<PasswordInput placeholder="Password" />);

    const toggle = screen.getByRole("button", { name: "Show password" });
    await user.click(toggle);
    await user.click(screen.getByRole("button", { name: "Hide password" }));

    expect(screen.getByPlaceholderText("Password")).toHaveAttribute(
      "type",
      "password",
    );
  });

  it("forwards additional props to the underlying input", async () => {
    const user = userEvent.setup();
    const handleChange = jest.fn();
    render(
      <PasswordInput
        placeholder="Password"
        value="hunter2"
        onChange={handleChange}
      />,
    );

    const input = screen.getByPlaceholderText("Password");
    expect(input).toHaveValue("hunter2");

    await user.type(input, "!");
    expect(handleChange).toHaveBeenCalled();
  });

  it("keeps the toggle button out of the tab order", () => {
    render(<PasswordInput placeholder="Password" />);

    expect(
      screen.getByRole("button", { name: "Show password" }),
    ).toHaveAttribute("tabIndex", "-1");
  });
});
