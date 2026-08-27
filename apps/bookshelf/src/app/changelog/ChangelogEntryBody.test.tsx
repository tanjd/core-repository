import { render, screen } from "@testing-library/react";

jest.mock("react-markdown", () => ({
  __esModule: true,
  default: ({ children }: { children: string }) => <div>{children}</div>,
}));

import { ChangelogEntryBody } from "@/app/changelog/ChangelogEntryBody";

describe("ChangelogEntryBody", () => {
  it("renders feature badges and hides migrations for members", () => {
    render(
      <ChangelogEntryBody
        showAdminDetails={false}
        body={`
### 🚀 Features

- **bookshelf:** add changelog page

### Database migrations

Includes migration **000014** — automatic on startup.

### ❤️ Thank You

- Jeddy Tan
`}
      />,
    );

    expect(screen.getByText("Features")).toBeInTheDocument();
    expect(screen.getByText(/add changelog page/)).toBeInTheDocument();
    expect(screen.queryByText("Database migrations")).not.toBeInTheDocument();
    expect(screen.queryByText("Thank You")).not.toBeInTheDocument();
  });

  it("renders migration alerts for admins", () => {
    render(
      <ChangelogEntryBody
        showAdminDetails
        body={`
### Database migrations

Includes migration **000014** — automatic on startup.
`}
      />,
    );

    expect(screen.getByRole("alert")).toHaveTextContent("000014");
  });
});
