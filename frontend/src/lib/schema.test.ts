import { describe, expect, it } from "vitest";
import { validateRule, type Rule } from "./schema";

const schema = [
  { field: "qps", type: "number", min: 1, max: 1000000 },
  { field: "min_requests", type: "number", min: 1, max: 100000 },
  { field: "open_timeout_ms", type: "number", min: 100, max: 600000 },
  { field: "half_open_probes", type: "number", min: 1, max: 32 },
  { field: "error_rate", type: "number", min: 0.01, max: 0.99 },
];

function base(): Rule {
  return {
    id: "x",
    service: "demo-gin",
    resource: "/work",
    method: "*",
    enabled: true,
    mode: "fixed",
    qps: 80,
    adaptive_min_qps: 10,
    adaptive_decrease: 0.7,
    adaptive_increase: 5,
    adaptive_latency_ms: 200,
    adaptive_error_rate: 0.3,
    adaptive_hysteresis: 3,
    error_rate: 0.5,
    min_requests: 20,
    open_timeout_ms: 5000,
    half_open_probes: 3,
    fallback: "default",
    version: 1,
    updated_at: "",
  };
}

describe("validateRule", () => {
  it("accepts a valid rule", () => {
    expect(validateRule(base(), schema)).toEqual({});
  });
  it("rejects empty service and out-of-range qps", () => {
    const r = base();
    r.service = "";
    r.qps = 0;
    const err = validateRule(r, schema);
    expect(err.service).toBeTruthy();
    expect(err.qps).toBeTruthy();
  });
});
