import createClient from "openapi-fetch";
import type { paths } from "./schema.js";

export const api = createClient<paths>({ baseUrl: "/api" });

export type Run = import("./schema.js").components["schemas"]["Run"];
export type StepState = import("./schema.js").components["schemas"]["StepState"];
export type RunStatus = import("./schema.js").components["schemas"]["RunStatus"];
export type StepStatus = import("./schema.js").components["schemas"]["StepStatus"];
