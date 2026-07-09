import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { PatientBanner } from "./PatientBanner";

describe("PatientBanner", () => {
  const defaultProps = {
    name: "John Smith",
    nhi: "ZAC1234",
    dob: "1990-01-15",
    gender: "Male",
  };

  it("renders patient name", () => {
    render(<PatientBanner {...defaultProps} />);
    expect(screen.getByText("John Smith")).toBeInTheDocument();
  });

  it("renders NHI with label", () => {
    render(<PatientBanner {...defaultProps} />);
    expect(screen.getByText("NHI: ZAC1234")).toBeInTheDocument();
  });

  it("renders date of birth", () => {
    render(<PatientBanner {...defaultProps} />);
    expect(screen.getByText("1990-01-15")).toBeInTheDocument();
  });

  it("renders gender", () => {
    render(<PatientBanner {...defaultProps} />);
    expect(screen.getByText("Male")).toBeInTheDocument();
  });

  it("renders optional address when provided", () => {
    render(<PatientBanner {...defaultProps} address="123 Test St" />);
    expect(screen.getByText("123 Test St")).toBeInTheDocument();
  });

  it("does not render address when not provided", () => {
    render(<PatientBanner {...defaultProps} />);
    expect(screen.queryByText("Address")).not.toBeInTheDocument();
  });

  it("renders optional GP name when provided", () => {
    render(<PatientBanner {...defaultProps} gpName="Dr Jones" />);
    expect(screen.getByText("Dr Jones")).toBeInTheDocument();
  });

  it("does not render GP name when not provided", () => {
    render(<PatientBanner {...defaultProps} />);
    expect(screen.queryByText("GP / Usual Doctor")).not.toBeInTheDocument();
  });

  it("has region role with accessible label", () => {
    render(<PatientBanner {...defaultProps} />);
    expect(screen.getByRole("region", { name: "Patient identification banner" })).toBeInTheDocument();
  });

  it("has accessible NHI label", () => {
    render(<PatientBanner {...defaultProps} />);
    expect(screen.getByLabelText("National Health Index: ZAC1234")).toBeInTheDocument();
  });

  it("shows health record indicator", () => {
    render(<PatientBanner {...defaultProps} />);
    expect(screen.getByText("Health record")).toBeInTheDocument();
  });

  it("merges custom className", () => {
    render(<PatientBanner {...defaultProps} className="my-banner" />);
    expect(screen.getByRole("region").className).toContain("my-banner");
  });
});
