export type FieldSchema = {
  field: string;
  type: string;
  required?: boolean;
  min?: number;
  max?: number;
  values?: string[];
};

export type Rule = {
  id: string;
  service: string;
  resource: string;
  method: string;
  enabled: boolean;
  mode: "fixed" | "adaptive";
  qps: number;
  adaptive_min_qps: number;
  adaptive_decrease: number;
  adaptive_increase: number;
  adaptive_latency_ms: number;
  adaptive_error_rate: number;
  adaptive_hysteresis: number;
  error_rate: number;
  min_requests: number;
  open_timeout_ms: number;
  half_open_probes: number;
  fallback: string;
  version: number;
  updated_at: string;
};

export type FieldErr = Record<string, string>;

export function validateRule(rule: Rule, schema: FieldSchema[]): FieldErr {
  const by = Object.fromEntries(schema.map((s) => [s.field, s]));
  const err: FieldErr = {};
  const need = (k: keyof Rule, label: string) => {
    const v = rule[k];
    if (v === undefined || v === null || String(v).trim() === "") err[k] = `${label}为必填`;
  };
  need("service", "服务");
  need("resource", "资源");
  const num = (k: keyof Rule, label: string) => {
    const spec = by[k];
    const v = Number(rule[k]);
    if (!Number.isFinite(v)) {
      err[k] = `${label}必须是数字`;
      return;
    }
    if (spec?.min != null && v < spec.min) err[k] = `${label}最小 ${spec.min}`;
    if (spec?.max != null && v > spec.max) err[k] = `${label}最大 ${spec.max}`;
  };
  ["qps", "min_requests", "open_timeout_ms", "half_open_probes", "error_rate"].forEach((k) =>
    num(k as keyof Rule, k),
  );
  if (rule.mode === "adaptive") {
    ["adaptive_min_qps", "adaptive_decrease", "adaptive_increase", "adaptive_latency_ms", "adaptive_error_rate", "adaptive_hysteresis"].forEach(
      (k) => num(k as keyof Rule, k),
    );
    if (rule.adaptive_min_qps > rule.qps) err.adaptive_min_qps = "下限不能高于 QPS";
  }
  return err;
}
