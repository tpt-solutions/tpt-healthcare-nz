import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { Table, type TableColumn } from "./Table";

interface TestRow extends Record<string, unknown> {
  name: string;
  age: number;
  status: string;
}

const columns: TableColumn<TestRow>[] = [
  { key: "name", header: "Name" },
  { key: "age", header: "Age" },
  { key: "status", header: "Status" },
];

const data: TestRow[] = [
  { name: "Alice", age: 30, status: "Active" },
  { name: "Bob", age: 25, status: "Inactive" },
];

describe("Table", () => {
  it("renders column headers", () => {
    render(<Table<TestRow> columns={columns} data={[]} />);
    expect(screen.getByText("Name")).toBeInTheDocument();
    expect(screen.getByText("Age")).toBeInTheDocument();
    expect(screen.getByText("Status")).toBeInTheDocument();
  });

  it("renders data rows", () => {
    render(<Table<TestRow> columns={columns} data={data} />);
    expect(screen.getByText("Alice")).toBeInTheDocument();
    expect(screen.getByText("30")).toBeInTheDocument();
    expect(screen.getByText("Bob")).toBeInTheDocument();
    expect(screen.getByText("25")).toBeInTheDocument();
  });

  it("shows empty message when no data", () => {
    render(<Table<TestRow> columns={columns} data={[]} />);
    expect(screen.getByText("No data to display.")).toBeInTheDocument();
  });

  it("shows custom empty message", () => {
    render(
      <Table<TestRow> columns={columns} data={[]} emptyMessage="No patients found." />
    );
    expect(screen.getByText("No patients found.")).toBeInTheDocument();
  });

  it("shows skeleton rows when loading", () => {
    render(<Table<TestRow> columns={columns} data={[]} loading />);
    // 3 skeleton rows × 3 columns = 9 pulse divs
    const { container } = render(
      <Table<TestRow> columns={columns} data={[]} loading />
    );
    const pulseDivs = container.querySelectorAll(".animate-pulse");
    expect(pulseDivs.length).toBe(9);
  });

  it("uses custom render function", () => {
    const customColumns: TableColumn<TestRow>[] = [
      { key: "name", header: "Name", render: (row) => <strong>{row.name}</strong> },
    ];
    render(<Table<TestRow> columns={customColumns} data={data} />);
    expect(screen.getByText("Alice").tagName).toBe("STRONG");
  });

  it("handles undefined values gracefully", () => {
    const sparseData = [{ name: "Alice" }] as TestRow[];
    render(<Table<TestRow> columns={columns} data={sparseData} />);
    expect(screen.getByText("Alice")).toBeInTheDocument();
  });
});
