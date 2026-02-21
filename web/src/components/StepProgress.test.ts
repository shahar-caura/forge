import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/svelte";
import StepProgress from "./StepProgress.svelte";
import type { StepState } from "../lib/api/client.js";

function makeSteps(statuses: Array<StepState["status"]>): StepState[] {
  const names = ["read plan", "create issue", "generate branch", "create worktree", "run agent"];
  return statuses.map((status, i) => ({
    name: names[i] || `step ${i}`,
    status,
  }));
}

describe("StepProgress", () => {
  it("renders all step names", () => {
    const steps = makeSteps(["completed", "running", "pending"]);
    render(StepProgress, { props: { steps } });

    expect(screen.getByText("read plan")).toBeInTheDocument();
    expect(screen.getByText("create issue")).toBeInTheDocument();
    expect(screen.getByText("generate branch")).toBeInTheDocument();
  });

  it("shows check mark for completed steps", () => {
    const steps = makeSteps(["completed"]);
    render(StepProgress, { props: { steps } });
    expect(screen.getByText("✓")).toBeInTheDocument();
  });

  it("shows cross for failed steps", () => {
    const steps: StepState[] = [{ name: "run agent", status: "failed", error: "exit code 1" }];
    render(StepProgress, { props: { steps } });
    expect(screen.getByText("✗")).toBeInTheDocument();
    expect(screen.getByText("exit code 1")).toBeInTheDocument();
  });

  it("shows pending circle for pending steps", () => {
    const steps = makeSteps(["pending"]);
    const { container } = render(StepProgress, { props: { steps } });
    expect(container.querySelector(".circle")).not.toBeNull();
  });

  it("shows spinner for running steps", () => {
    const steps = makeSteps(["running"]);
    const { container } = render(StepProgress, { props: { steps } });
    expect(container.querySelector(".spinner")).not.toBeNull();
  });
});
