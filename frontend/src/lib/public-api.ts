const internalOrigin = process.env.API_INTERNAL_ORIGIN ?? process.env.API_ORIGIN ?? "http://localhost:8080";

export async function publicApiFetch<T>(path: string): Promise<T | null> {
  const response = await fetch(`${internalOrigin}${path}`, {
    next: { revalidate: 30 }
  });
  if (response.status === 404) {
    return null;
  }
  if (!response.ok) {
    throw new Error(`Public API request failed: ${response.status}`);
  }
  return (await response.json()) as T;
}
