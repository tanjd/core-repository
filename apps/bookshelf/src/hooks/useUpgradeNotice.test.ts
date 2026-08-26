import { renderHook, act } from "@testing-library/react";
import { useUpgradeNotice } from "./useUpgradeNotice";

describe("useUpgradeNotice", () => {
  const originalVersion = process.env.NEXT_PUBLIC_VERSION;

  beforeEach(() => {
    localStorage.clear();
    process.env.NEXT_PUBLIC_VERSION = "0.22.0";
  });

  afterEach(() => {
    process.env.NEXT_PUBLIC_VERSION = originalVersion;
  });

  it("does not show a notice on first visit", () => {
    const { result } = renderHook(() => useUpgradeNotice());
    expect(result.current.visible).toBe(false);
  });

  it("shows a notice when the running version is newer than the stored one", () => {
    localStorage.setItem("bookshelf_last_seen_app_version", "0.21.0");

    const { result } = renderHook(() => useUpgradeNotice());
    expect(result.current.visible).toBe(true);
    expect(result.current.version).toBe("0.22.0");
  });

  it("persists the current version on dismiss", () => {
    localStorage.setItem("bookshelf_last_seen_app_version", "0.21.0");

    const { result } = renderHook(() => useUpgradeNotice());
    act(() => {
      result.current.dismiss();
    });

    expect(result.current.visible).toBe(false);
    expect(localStorage.getItem("bookshelf_last_seen_app_version")).toBe(
      "0.22.0",
    );
  });
});
