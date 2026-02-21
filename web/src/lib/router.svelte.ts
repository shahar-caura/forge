let currentHash = $state(window.location.hash || "#/");

window.addEventListener("hashchange", () => {
  currentHash = window.location.hash || "#/";
});

export function navigate(path: string) {
  window.location.hash = path;
}

export interface RouteMatch {
  route: string;
  params: Record<string, string>;
}

export function match(): RouteMatch {
  const hash = currentHash.slice(1) || "/"; // strip leading #

  const runDetail = hash.match(/^\/runs\/(.+)$/);
  if (runDetail) {
    return { route: "run-detail", params: { id: runDetail[1] } };
  }

  if (hash === "/stats") {
    return { route: "stats", params: {} };
  }

  return { route: "runs", params: {} };
}
