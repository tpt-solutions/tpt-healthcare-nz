import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { NHIInput } from "./NHIInput";

describe("NHIInput", () => {
  it("renders with default label", () => {
    render(<NHIInput value="" onChange={() => {}} />);
    expect(screen.getByLabelText("NHI Number")).toBeInTheDocument();
  });

  it("renders with custom label", () => {
    render(<NHIInput label="Patient NHI" value="" onChange={() => {}} />);
    expect(screen.getByLabelText("Patient NHI")).toBeInTheDocument();
  });

  it("renders hint text by default", () => {
    render(<NHIInput value="" onChange={() => {}} />);
    expect(screen.getByText(/National Health Index number/)).toBeInTheDocument();
  });

  it("shows required indicator when required", () => {
    render(<NHIInput value="" onChange={() => {}} required />);
    expect(screen.getByText("*")).toBeInTheDocument();
  });

  it("calls onChange when typing", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<NHIInput value="" onChange={onChange} />);
    await user.type(screen.getByRole("textbox"), "ABC");
    expect(onChange).toHaveBeenCalledTimes(3);
  });

  it("shows valid state for valid NHI", async () => {
    const user = userEvent.setup();
    render(<NHIInput value="AAA0004" onChange={() => {}} />);
    const input = screen.getByRole("textbox");
    // Type to trigger touched state
    await user.click(input);
    await user.tab();
    expect(screen.getByText("Valid NHI number.")).toBeInTheDocument();
  });

  it("shows error for invalid NHI after blur", async () => {
    const user = userEvent.setup();
    render(<NHIInput value="INVALID" onChange={() => {}} />);
    const input = screen.getByRole("textbox");
    await user.click(input);
    await user.tab();
    expect(screen.getByRole("alert")).toHaveTextContent("Invalid NHI format");
  });

  it("does not show error before touch", () => {
    render(<NHIInput value="INVALID" onChange={() => {}} />);
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("applies valid border styling", async () => {
    const user = userEvent.setup();
    render(<NHIInput value="AAA0004" onChange={() => {}} />);
    const input = screen.getByRole("textbox");
    await user.click(input);
    await user.tab();
    expect(input.className).toContain("border-green-500");
  });

  it("applies error border styling after touch", async () => {
    const user = userEvent.setup();
    render(<NHIInput value="X" onChange={() => {}} />);
    const input = screen.getByRole("textbox");
    await user.click(input);
    await user.tab();
    expect(input.className).toContain("border-red-400");
  });

  it("can be disabled", () => {
    render(<NHIInput value="AAA0004" onChange={() => {}} disabled />);
    expect(screen.getByRole("textbox")).toBeDisabled();
  });

  it("sets maxLength to 10", () => {
    render(<NHIInput value="" onChange={() => {}} />);
    expect(screen.getByRole("textbox")).toHaveAttribute("maxlength", "10");
  });

  it("uppercase-transforms the displayed value via CSS class", () => {
    render(<NHIInput value="abc1234" onChange={() => {}} />);
    expect(screen.getByRole("textbox").className).toContain("uppercase");
  });
});
