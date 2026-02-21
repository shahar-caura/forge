import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/svelte";
import RunDetail from "./RunDetail.svelte";
import type { Run } from "../lib/api/client.js";

vi.stubGlobal("fetch", vi.fn());

function makeRun(overrides: Partial<Run> = {}): Run {
  return {
    id: "detail-1",
    plan_path: "plans/auth.md",
    plan_title: "Auth Feature",
    status: "active",
    branch: "forge/auth-feature",
    created_at: "2026-02-20T10:00:00Z",
    updated_at: "2026-02-20T10:05:00Z",
    pr_url: "https://github.com/org/repo/pull/7",
    pr_number: 7,
    steps: [
      { name: "read plan", status: "completed" },
      { name: "create issue", status: "running" },
    ],
    ...overrides,
  };
}

describe("RunDetail", () => {
  beforeEach(async () => {
    const { getRuns } = await import("../lib/state/runs.svelte.js");
    getRuns().length = 0;
  });

  it("shows 'Run not found' for missing ID", () => {
    render(RunDetail, { props: { id: "missing" } });
    expect(screen.getByText("Run not found.")).toBeInTheDocument();
  });

  it("renders run metadata", async () => {
    const { upsert } = await import("../lib/state/runs.svelte.js");
    upsert(makeRun());

    render(RunDetail, { props: { id: "detail-1" } });
    expect(screen.getByText("Auth Feature")).toBeInTheDocument();
    expect(screen.getByText("forge/auth-feature")).toBeInTheDocument();
    expect(screen.getByText("detail-1")).toBeInTheDocument();
  });

  it("renders PR link", async () => {
    const { upsert } = await import("../lib/state/runs.svelte.js");
    upsert(makeRun());

    render(RunDetail, { props: { id: "detail-1" } });
    const link = screen.getByText("PR #7");
    expect(link.closest("a")).toHaveAttribute("href", "https://github.com/org/repo/pull/7");
  });

  it("renders step progress", async () => {
    const { upsert } = await import("../lib/state/runs.svelte.js");
    upsert(makeRun());

    render(RunDetail, { props: { id: "detail-1" } });
    expect(screen.getByText("read plan")).toBeInTheDocument();
    expect(screen.getByText("create issue")).toBeInTheDocument();
  });

  it("shows back button", async () => {
    const { upsert } = await import("../lib/state/runs.svelte.js");
    upsert(makeRun());

    render(RunDetail, { props: { id: "detail-1" } });
    expect(screen.getByText("← Runs")).toBeInTheDocument();
  });
});
