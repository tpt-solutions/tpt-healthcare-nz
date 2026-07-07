import React, { createContext, useContext, useMemo } from 'react';
import { useAuth } from './AuthContext';

// ---------------------------------------------------------------------------
// Minimal typed API client
// In production this would be generated from the OpenAPI spec via
// @tpt/api-client (openapi-typescript). For now we define a slim hand-rolled
// client that wraps fetch with auth headers and base URL.
// ---------------------------------------------------------------------------

export interface RequestOptions extends Omit<RequestInit, 'body'> {
  params?: Record<string, string | number | boolean | undefined>;
  body?: unknown;
}

export class TptApiClient {
  private readonly baseUrl: string;
  private readonly getToken: () => string | null;
  private readonly getTenantId: () => string | null;

  constructor(baseUrl: string, getToken: () => string | null, getTenantId: () => string | null) {
    this.baseUrl = baseUrl;
    this.getToken = getToken;
    this.getTenantId = getTenantId;
  }

  private buildUrl(path: string, params?: RequestOptions['params']): string {
    const url = new URL(`${this.baseUrl}${path}`, window.location.origin);
    if (params) {
      for (const [key, value] of Object.entries(params)) {
        if (value !== undefined) {
          url.searchParams.set(key, String(value));
        }
      }
    }
    return url.toString();
  }

  async request<T>(path: string, options: RequestOptions = {}): Promise<T> {
    const { params, body, headers: extraHeaders, ...rest } = options;
    const token = this.getToken();
    const tenantId = this.getTenantId();

    const headers: HeadersInit = {
      'Content-Type': 'application/json',
      Accept: 'application/fhir+json, application/json',
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
      ...(tenantId ? { 'X-Tenant-ID': tenantId } : {}),
      ...extraHeaders,
    };

    const response = await fetch(this.buildUrl(path, params), {
      ...rest,
      headers,
      body: body !== undefined ? JSON.stringify(body) : undefined,
    });

    if (!response.ok) {
      const errorBody = await response.json().catch(() => ({})) as { message?: string };
      throw new ApiError(
        errorBody.message ?? `HTTP ${response.status} ${response.statusText}`,
        response.status,
      );
    }

    // 204 No Content
    if (response.status === 204) {
      return undefined as unknown as T;
    }

    return response.json() as Promise<T>;
  }

  get<T>(path: string, options?: RequestOptions): Promise<T> {
    return this.request<T>(path, { ...options, method: 'GET' });
  }

  post<T>(path: string, body: unknown, options?: RequestOptions): Promise<T> {
    return this.request<T>(path, { ...options, method: 'POST', body });
  }

  put<T>(path: string, body: unknown, options?: RequestOptions): Promise<T> {
    return this.request<T>(path, { ...options, method: 'PUT', body });
  }

  patch<T>(path: string, body: unknown, options?: RequestOptions): Promise<T> {
    return this.request<T>(path, { ...options, method: 'PATCH', body });
  }

  delete<T>(path: string, options?: RequestOptions): Promise<T> {
    return this.request<T>(path, { ...options, method: 'DELETE' });
  }
}

export class ApiError extends Error {
  constructor(
    message: string,
    public readonly statusCode: number,
  ) {
    super(message);
    this.name = 'ApiError';
  }
}

// ---------------------------------------------------------------------------
// Context
// ---------------------------------------------------------------------------

const ApiContext = createContext<TptApiClient | null>(null);

export function ApiProvider({ children }: { children: React.ReactNode }) {
  const { getToken, getTenantId } = useAuth();

  const client = useMemo(
    () => new TptApiClient('/api/v1', getToken, getTenantId),
    [getToken, getTenantId],
  );

  return <ApiContext.Provider value={client}>{children}</ApiContext.Provider>;
}

export function useApi(): TptApiClient {
  const ctx = useContext(ApiContext);
  if (!ctx) {
    throw new Error('useApi must be used within an <ApiProvider>');
  }
  return ctx;
}
