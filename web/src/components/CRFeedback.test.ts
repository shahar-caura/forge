import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/svelte";
import CRFeedback from "./CRFeedback.svelte";

describe("CRFeedback", () => {
  it("renders nothing when no CR data", () => {
    const { container } = render(CRFeedback, {
      props: { crFeedback: undefined, crFixSummary: undefined },
    });
    expect(container.querySelector(".cr-section")).toBeNull();
  });

  it("renders feedback markdown", () => {
    render(CRFeedback, {
      props: {
        crFeedback: "**Bold** feedback",
        crFixSummary: undefined,
      },
    });
    expect(screen.getByText("Review Feedback")).toBeInTheDocument();
    expect(screen.getByText("Bold")).toBeInTheDocument();
  });

  it("renders fix summary markdown", () => {
    render(CRFeedback, {
      props: {
        crFeedback: undefined,
        crFixSummary: "Fixed the `bug`",
      },
    });
    expect(screen.getByText("Fix Summary")).toBeInTheDocument();
    expect(screen.getByText("bug")).toBeInTheDocument();
  });

  it("renders both sections when both present", () => {
    render(CRFeedback, {
      props: {
        crFeedback: "Some feedback",
        crFixSummary: "Some fix",
      },
    });
    expect(screen.getByText("Review Feedback")).toBeInTheDocument();
    expect(screen.getByText("Fix Summary")).toBeInTheDocument();
  });
});
