import { getCredential, getHost } from "./config.js";

export class ApiError extends Error {
  constructor(
    public status: number,
    message: string
  ) {
    super(message);
    this.name = "ApiError";
  }
}

export class ScrapsClient {
  private host: string;
  private apiKey: string;

  constructor(host?: string, apiKey?: string) {
    this.host = host || getHost();
    const cred = getCredential(this.host);
    this.apiKey = apiKey || cred?.api_key || "";
  }

  getHost(): string {
    return this.host;
  }

  getApiKey(): string {
    return this.apiKey;
  }

  hasAuth(): boolean {
    return !!this.apiKey;
  }

  async request<T = any>(
    method: string,
    path: string,
    body?: any
  ): Promise<T> {
    const headers: Record<string, string> = {
      "Content-Type": "application/json",
    };
    if (this.apiKey) {
      headers["Authorization"] = `Bearer ${this.apiKey}`;
    }

    const res = await fetch(`${this.host}${path}`, {
      method,
      headers,
      body: body ? JSON.stringify(body) : undefined,
    });

    if (!res.ok) {
      let message = res.statusText;
      try {
        const err = await res.json();
        message = err.error || message;
      } catch {
        // ignore parse error
      }
      throw new ApiError(res.status, message);
    }

    const text = await res.text();
    if (!text) return undefined as T;
    return JSON.parse(text);
  }

  // Convenience methods
  get<T = any>(path: string): Promise<T> {
    return this.request<T>("GET", path);
  }

  post<T = any>(path: string, body?: any): Promise<T> {
    return this.request<T>("POST", path, body);
  }

  put<T = any>(path: string, body?: any): Promise<T> {
    return this.request<T>("PUT", path, body);
  }

  patch<T = any>(path: string, body?: any): Promise<T> {
    return this.request<T>("PATCH", path, body);
  }

  delete<T = any>(path: string, body?: any): Promise<T> {
    return this.request<T>("DELETE", path, body);
  }
}

let defaultClient: ScrapsClient | null = null;

export function getClient(): ScrapsClient {
  if (!defaultClient) {
    defaultClient = new ScrapsClient();
  }
  return defaultClient;
}

export function requireAuth(): ScrapsClient {
  const client = getClient();
  if (!client.hasAuth()) {
    console.error("Error: Not logged in. Run 'scraps login' first.");
    process.exit(1);
  }
  return client;
}
