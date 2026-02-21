import { describe, it, expect, vi, afterEach } from "vitest";
import { relativeTime } from "./time.js";

describe("relativeTime", () => {
  afterEach(() => {
    vi.useRealTimers();
  });

  it("returns 'just now' for < 5 seconds ago", () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-02-20T10:00:05Z"));
    expect(relativeTime("2026-02-20T10:00:02Z")).toBe("just now");
  });

  it("returns seconds for < 1 minute", () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-02-20T10:00:30Z"));
    expect(relativeTime("2026-02-20T10:00:00Z")).toBe("30s ago");
  });

  it("returns minutes for < 1 hour", () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-02-20T10:15:00Z"));
    expect(relativeTime("2026-02-20T10:00:00Z")).toBe("15m ago");
  });

  it("returns hours for < 1 day", () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-02-20T13:00:00Z"));
    expect(relativeTime("2026-02-20T10:00:00Z")).toBe("3h ago");
  });

  it("returns days for >= 1 day", () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-02-22T10:00:00Z"));
    expect(relativeTime("2026-02-20T10:00:00Z")).toBe("2d ago");
  });
});
