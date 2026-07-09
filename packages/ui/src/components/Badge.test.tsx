import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { Badge } from "./Badge";

describe("Badge", () => {
  it("renders children text", () => {
    render(<Badge>Active</Badge>);
    expect(screen.getByText("Active")).toBeInTheDocument();
  });

  it("renders with neutral variant by default", () => {
    render(<Badge>Neutral</Badge>);
    const badge = screen.getByText("Neutral").closest("span")!;
    expect(badge.className).toContain("bg-secondary-100");
  });

  it("renders success variant", () => {
    render(<Badge variant="success">Success</Badge>);
    const badge = screen.getByText("Success").closest("span")!;
    expect(badge.className).toContain("bg-green-100");
  });

  it("renders warning variant", () => {
    render(<Badge variant="warning">Warning</Badge>);
    const badge = screen.getByText("Warning").closest("span")!;
    expect(badge.className).toContain("bg-yellow-100");
  });

  it("renders error variant", () => {
    render(<Badge variant="error">Error</Badge>);
    const badge = screen.getByText("Error").closest("span")!;
    expect(badge.className).toContain("bg-red-100");
  });

  it("renders info variant", () => {
    render(<Badge variant="info">Info</Badge>);
    const badge = screen.getByText("Info").closest("span")!;
    expect(badge.className).toContain("bg-primary-100");
  });

  it("includes a decorative dot", () => {
    const { container } = render(<Badge>Test</Badge>);
    const dot = container.querySelector(".rounded-full.bg-secondary-400");
    expect(dot).toBeInTheDocument();
    expect(dot).toHaveAttribute("aria-hidden", "true");
  });

  it("merges custom className", () => {
    render(<Badge className="my-badge">Custom</Badge>);
    const badge = screen.getByText("Custom").closest("span")!;
    expect(badge.className).toContain("my-badge");
  });
});
