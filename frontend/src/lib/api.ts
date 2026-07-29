import type { APIErrorBody } from "@/lib/types";

export class APIError extends Error {
  constructor(
    public readonly status: number,
    public readonly code: string,
    message: string
  ) {
    super(message);
    this.name = "APIError";
  }
}

export async function apiFetch<T>(path: string, init?: RequestInit): Promise<T> {
  const headers = new Headers(init?.headers);
  if (init?.body && !(init.body instanceof FormData) && !headers.has("Content-Type")) {
    headers.set("Content-Type", "application/json");
  }
  const response = await fetch(path, {
    ...init,
    headers,
    credentials: "same-origin"
  });
  if (!response.ok) {
    let body: APIErrorBody | undefined;
    try {
      body = (await response.json()) as APIErrorBody;
    } catch {
      body = undefined;
    }
    throw new APIError(response.status, body?.error.code ?? "REQUEST_FAILED", body?.error.message ?? `Request failed with status ${response.status}`);
  }
  if (response.status === 204) {
    return undefined as T;
  }
  return (await response.json()) as T;
}

export function jsonBody(value: unknown): RequestInit {
  return { body: JSON.stringify(value) };
}
