export type Envelope<T> = { data?: T; error?: { code: string; message: string; details?: { field: string; message: string }[] } };

async function req<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, {
    ...init,
    headers: { "Content-Type": "application/json", ...(init?.headers || {}) },
  });
  if (res.status === 204) return undefined as T;
  const body = (await res.json()) as Envelope<T>;
  if (!res.ok || body.error) {
    const msg = body.error?.message || `HTTP ${res.status}`;
    throw Object.assign(new Error(msg), { details: body.error?.details, status: res.status });
  }
  return body.data as T;
}

export const api = {
  overview: () => req<any>("/api/v1/overview"),
  metrics: (q: string) => req<any[]>(`/api/v1/metrics${q}`),
  rules: () => req<{ version: number; rules: any[] }>("/api/v1/rules"),
  schema: () => req<any[]>("/api/v1/schema"),
  saveRule: (rule: any) =>
    rule.id
      ? req(`/api/v1/rules/${encodeURIComponent(rule.id)}`, { method: "PUT", body: JSON.stringify(rule) })
      : req("/api/v1/rules", { method: "POST", body: JSON.stringify(rule) }),
  patchRule: (id: string, patch: any) =>
    req(`/api/v1/rules/${encodeURIComponent(id)}`, { method: "PATCH", body: JSON.stringify(patch) }),
  resetRule: (id: string) => req(`/api/v1/rules/${encodeURIComponent(id)}/reset`, { method: "POST" }),
};
