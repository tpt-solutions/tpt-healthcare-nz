import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { Card } from "./Card";

describe("Card", () => {
  it("renders children", () => {
    render(<Card>Card content</Card>);
    expect(screen.getByText("Card content")).toBeInTheDocument();
  });

  it("renders title when provided", () => {
    render(<Card title="Patient Details">Content</Card>);
    expect(screen.getByText("Patient Details")).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Patient Details" })).toBeInTheDocument();
  });

  it("does not render header when no title or actions", () => {
    const { container } = render(<Card>Content</Card>);
    expect(container.querySelector("h3")).not.toBeInTheDocument();
  });

  it("renders actions when provided", () => {
    render(<Card title="T" actions={<button>Save</button>}>Content</Card>);
    expect(screen.getByRole("button", { name: "Save" })).toBeInTheDocument();
  });

  it("renders header with title only (no actions)", () => {
    render(<Card title="Title">Content</Card>);
    expect(screen.getByRole("heading", { name: "Title" })).toBeInTheDocument();
  });

  it("merges custom className", () => {
    const { container } = render(<Card className="my-card">Content</Card>);
    expect(container.firstElementChild!.className).toContain("my-card");
  });
});
