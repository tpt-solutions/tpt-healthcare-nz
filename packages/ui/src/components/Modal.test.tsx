import { render, screen, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { Modal } from "./Modal";

describe("Modal", () => {
  const onClose = vi.fn();

  beforeEach(() => {
    onClose.mockClear();
    Object.defineProperty(HTMLDialogElement.prototype, "showModal", {
      value: vi.fn(),
      writable: true,
      configurable: true,
    });
    Object.defineProperty(HTMLDialogElement.prototype, "close", {
      value: vi.fn(),
      writable: true,
      configurable: true,
    });
  });

  it("renders children when open", () => {
    render(
      <Modal open={true} onClose={onClose} title="Test Modal">
        <p>Modal content</p>
      </Modal>,
    );
    expect(screen.getByText("Modal content")).toBeInTheDocument();
  });

  it("renders title", () => {
    render(
      <Modal open={true} onClose={onClose} title="Confirm Action">
        <p>Content</p>
      </Modal>,
    );
    expect(screen.getByText("Confirm Action")).toBeInTheDocument();
  });

  it("calls onClose when close button clicked", () => {
    render(
      <Modal open={true} onClose={onClose} title="Closeable">
        <p>Content</p>
      </Modal>,
    );
    fireEvent.click(screen.getByLabelText("Close dialog"));
    expect(onClose).toHaveBeenCalledOnce();
  });

  it("calls onClose when backdrop clicked", () => {
    render(
      <Modal open={true} onClose={onClose} title="Backdrop">
        <p>Content</p>
      </Modal>,
    );
    const dialog = document.querySelector("dialog")!;
    const rect = { left: 100, right: 400, top: 100, bottom: 400 };
    vi.spyOn(dialog, "getBoundingClientRect").mockReturnValue(rect as DOMRect);
    fireEvent.click(dialog, { clientX: 10, clientY: 10 });
    expect(onClose).toHaveBeenCalledOnce();
  });

  it("does not call onClose when clicking inside panel", () => {
    render(
      <Modal open={true} onClose={onClose} title="Panel click">
        <p>Content</p>
      </Modal>,
    );
    fireEvent.click(screen.getByText("Content"));
    expect(onClose).not.toHaveBeenCalled();
  });

  it("applies size class", () => {
    const { rerender } = render(
      <Modal open={true} onClose={onClose} title="Small" size="sm">
        <p>Content</p>
      </Modal>,
    );
    const dialog = document.querySelector("dialog")!;
    expect(dialog.className).toContain("max-w-sm");

    rerender(
      <Modal open={true} onClose={onClose} title="Large" size="lg">
        <p>Content</p>
      </Modal>,
    );
    expect(dialog.className).toContain("max-w-2xl");
  });

  it("sets aria-labelledby on dialog", () => {
    render(
      <Modal open={true} onClose={onClose} title="Accessible">
        <p>Content</p>
      </Modal>,
    );
    const dialog = document.querySelector("dialog")!;
    const labelledBy = dialog.getAttribute("aria-labelledby");
    expect(labelledBy).toBeTruthy();
    expect(document.getElementById(labelledBy!)).toHaveTextContent("Accessible");
  });
});
