import { render, screen } from "@testing-library/react";
import App from "./App";

test("renders home page greeting", () => {
  render(<App />);
  expect(screen.getByText(/Hello! I am Jeddy/i)).toBeInTheDocument();
});
