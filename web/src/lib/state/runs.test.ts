import { describe, it, expect, vi, beforeEach } from "vitest";
import type { Run } from "../api/client.js";

// Mock the API client module
vi.mock("../api/client.js", () => {
  const apiMock = {
    GET: vi.fn(),
  };
  return {
    api: apiMock,
  };
});

const { api } = await import("../api/client.js");
const apiGet = vi.mocked(api.GET);

const { getRuns, getFilteredRuns, setStatusFilter, upsert, fetchRuns, findRun, getError } =
  await import("./runs.svelte.js");

function makeRun(overrides: Partial<Run> = {}): Run {
  return {
    id: "test-1",
    plan_path: "plans/auth.md",
    status: "active",
    created_at: "2026-02-20T10:00:00Z",
    updated_at: "2026-02-20T10:05:00Z",
    steps: [
      { name: "read plan", status: "completed" },
      { name: "create issue", status: "running" },
      { name: "generate branch", status: "pending" },
    ],
    ...overrides,
  };
}

describe("runs state", () => {
  beforeEach(() => {
    getRuns().length = 0;
    apiGet.mockReset();
  });

  describe("upsert", () => {
    it("adds a new run", () => {
      upsert(makeRun({ id: "new-1" }));
      expect(getRuns()).toHaveLength(1);
      expect(getRuns()[0].id).toBe("new-1");
    });

    it("updates an existing run", () => {
      upsert(makeRun({ id: "upd-1", status: "active" }));
      expect(getRuns()[0].status).toBe("active");

      upsert(makeRun({ id: "upd-1", status: "completed" }));
      expect(getRuns()).toHaveLength(1);
      expect(getRuns()[0].status).toBe("completed");
    });

    it("prepends new runs (newest first)", () => {
      upsert(makeRun({ id: "a" }));
      upsert(makeRun({ id: "b" }));
      expect(getRuns()[0].id).toBe("b");
    });
  });

  describe("findRun", () => {
    it("returns the run with matching id", () => {
      upsert(makeRun({ id: "find-me" }));
      expect(findRun("find-me")?.id).toBe("find-me");
    });

    it("returns undefined for missing id", () => {
      expect(findRun("nope")).toBeUndefined();
    });
  });

  describe("filter", () => {
    it("returns all runs when filter is 'all'", () => {
      upsert(makeRun({ id: "a", status: "active" }));
      upsert(makeRun({ id: "b", status: "completed" }));
      setStatusFilter("all");
      expect(getFilteredRuns()).toHaveLength(2);
    });

    it("filters by status", () => {
      upsert(makeRun({ id: "a", status: "active" }));
      upsert(makeRun({ id: "b", status: "completed" }));
      upsert(makeRun({ id: "c", status: "failed" }));
      setStatusFilter("failed");
      expect(getFilteredRuns()).toHaveLength(1);
      expect(getFilteredRuns()[0].id).toBe("c");
    });
  });

  describe("fetchRuns", () => {
    it("populates runs from API", async () => {
      const runs = [makeRun({ id: "fetch-1" }), makeRun({ id: "fetch-2" })];
      apiGet.mockResolvedValueOnce({
        data: { runs, total: 2 },
        error: undefined,
        response: new Response(),
      });

      await fetchRuns();
      expect(getRuns()).toHaveLength(2);
      expect(getRuns()[0].id).toBe("fetch-1");
    });

    it("sets error on fetch failure", async () => {
      apiGet.mockResolvedValueOnce({
        data: undefined,
        error: { code: 500, message: "Internal error" },
        response: new Response(null, { status: 500 }),
      });

      await fetchRuns();
      expect(getError()).toBe("Failed to fetch runs");
    });
  });
});
