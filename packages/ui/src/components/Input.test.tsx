import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { Input } from "./Input";

describe("Input", () => {
  it("renders label", () => {
    render(<Input label="Email" />);
    expect(screen.getByLabelText("Email")).toBeInTheDocument();
  });

  it("renders input element", () => {
    render(<Input label="Name" />);
    expect(screen.getByRole("textbox", { name: "Name" })).toBeInTheDocument();
  });

  it("shows required indicator", () => {
    render(<Input label="Email" required />);
    expect(screen.getByText("*")).toBeInTheDocument();
    expect(screen.getByRole("textbox")).toHaveAttribute("aria-required", "true");
  });

  it("does not show required indicator by default", () => {
    render(<Input label="Email" />);
    expect(screen.queryByText("*")).not.toBeInTheDocument();
  });

  it("shows error message", () => {
    render(<Input label="Email" error="Invalid email address" />);
    expect(screen.getByRole("alert")).toHaveTextContent("Invalid email address");
    expect(screen.getByRole("textbox")).toHaveAttribute("aria-invalid", "true");
  });

  it("links error to input via aria-describedby", () => {
    render(<Input label="Email" error="Required" />);
    const input = screen.getByRole("textbox");
    const errorId = input.getAttribute("aria-describedby");
    expect(errorId).toBeTruthy();
    expect(document.getElementById(errorId!)).toHaveTextContent("Required");
  });

  it("shows hint text", () => {
    render(<Input label="Password" hint="Must be 8+ characters" />);
    expect(screen.getByText("Must be 8+ characters")).toBeInTheDocument();
  });

  it("links hint to input via aria-describedby", () => {
    render(<Input label="Password" hint="At least 8 characters" />);
    const input = screen.getByRole("textbox");
    const desc = input.getAttribute("aria-describedby");
    expect(desc).toBeTruthy();
    expect(document.getElementById(desc!)).toHaveTextContent("At least 8 characters");
  });

  it("passes through input props", () => {
    render(<Input label="Email" placeholder="you@example.com" type="email" />);
    const input = screen.getByRole("textbox");
    expect(input).toHaveAttribute("placeholder", "you@example.com");
    expect(input).toHaveAttribute("type", "email");
  });

  it("merges custom className on input", () => {
    render(<Input label="Name" className="my-input" />);
    expect(screen.getByRole("textbox").className).toContain("my-input");
  });
});
