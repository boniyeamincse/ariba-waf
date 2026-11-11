export type ApiErrorPayload = {
  code?: string;
  message?: string;
};

export type ApiMeta = {
  requestId?: string;
  nextCursor?: string | null;
  desiredGeneration?: number;
};

export type ApiResponse<T> = {
  data: T;
  meta?: ApiMeta;
};

export class ApiClientError extends Error {
  status: number;
  code: string;
  requestId?: string;

  constructor(message: string, status: number, code: string, requestId?: string) {
    super(message);
    this.name = "ApiClientError";
    this.status = status;
    this.code = code;
    this.requestId = requestId;
  }
}

export type Role = "admin" | "operator" | "viewer";

export type User = {
  id: string;
  username: string;
  displayName: string;
  role: Role;
  enabled: boolean;
  version: number;
};

export type MeResponse = {
  expiresAt: string;
  user: User;
};

export type SessionItem = {
  id: string;
  current: boolean;
  createdAt: string;
  lastSeenAt: string;
  expiresAt: string;
};

export type AuthPermissions = {
  role: Role;
  permissions: string[];
};

export type AuthActivity = {
  id: number;
  action: string;
  targetId?: string;
  at: string;
};

export type SetupStatus = {
  enrollmentRequired: boolean;
};

export type ConfigurationStatus = {
  desiredGeneration: number;
  appliedRevisionId: string | null;
  pendingChanges: boolean;
  currentJob: { id: string; status: string } | null;
  activationExecutorAvailable: boolean;
};

export type EngineMode = "off" | "detectionOnly" | "blocking";
export type OversizeAction = "reject" | "processPartial";
export type ParserErrorAction = "reject" | "logOnly";

export type PolicySpec = {
  name: string;
  engineMode: EngineMode;
  crs: {
    paranoiaLevel: number;
    inboundAnomalyThreshold: number;
    outboundAnomalyThreshold: number;
  };
  inspection: {
    requestBodyEnabled: boolean;
    requestBodyLimitBytes: number;
    requestBodyNoFilesLimitBytes: number;
    oversizeAction: OversizeAction;
    parserErrorAction: ParserErrorAction;
    responseBodyEnabled: boolean;
  };
  rateLimit: {
    enabled: boolean;
    requestsPerSecond: number;
    burst: number;
  };
};

export type Policy = PolicySpec & {
  id: string;
  version: number;
  desiredGeneration: number;
  appliedRevisionId: string | null;
  pendingChanges: boolean;
};

export type Upstream = {
  scheme: "http" | "https";
  host: string;
  port: number;
};

export type WebsiteSpec = {
  name: string;
  domains: string[];
  enabled: boolean;
  listeners: {
    http: boolean;
    https: boolean;
    redirectHttpToHttps: boolean;
  };
  certificateId: string | null;
  policyId: string;
  upstreams: Upstream[];
  upstreamTls: {
    verify: boolean;
    serverName: string;
    trustSource: "system";
    trustBundleId: string | null;
  };
  timeouts: {
    connectSeconds: number;
    readSeconds: number;
  };
};

export type Website = WebsiteSpec & {
  id: string;
  version: number;
  desiredGeneration: number;
  appliedRevisionId: string | null;
  pendingChanges: boolean;
};

export type ProxySettings = {
  allowedCidrs: string[];
  trustedProxyCidrs: string[];
  version: number;
  desiredGeneration: number;
  appliedRevisionId: string | null;
  pendingChanges: boolean;
};

export type EventMatch = {
  ruleId: number | string;
  severity?: string;
  message?: string;
};

export type EventRecord = {
  id: string;
  transactionId?: string;
  timestamp: string;
  siteId?: string;
  siteName?: string;
  appliedRevisionId?: string;
  engineMode?: EngineMode;
  clientIp?: string;
  method?: string;
  path?: string;
  statusCode?: number;
  action?: "allowed" | "blocked" | "rejected" | "unknown" | string;
  inspectionComplete?: boolean;
  matches?: EventMatch[];
  ruleId?: string;
  message?: string;
};

export type HealthProbeResult = {
  live: { ok: boolean; status: number; detail: string };
  ready: { ok: boolean; status: number; detail: string; code?: string };
  checkedAt: string;
};

type FetchOptions = {
  method?: "GET" | "POST" | "PATCH" | "DELETE";
  body?: unknown;
  csrfToken?: string;
  idempotencyKey?: string;
  ifMatch?: string;
};

export async function getCsrfToken(): Promise<string> {
  const response = await fetch("/api/v1/auth/csrf", {
    method: "GET",
    credentials: "include",
    cache: "no-store"
  });

  if (!response.ok) {
    throw new ApiClientError("Unable to obtain CSRF token", response.status, "CSRF_FETCH_FAILED");
  }

  const payload = (await response.json()) as ApiResponse<{ csrfToken: string }>;
  return payload.data.csrfToken;
}

export async function apiRequestWithMeta<T>(
  path: string,
  options: FetchOptions = {}
): Promise<{ data: T; meta?: ApiMeta; etag: string | null; requestId: string | null }> {
  const headers = new Headers({
    Accept: "application/json"
  });

  if (options.body !== undefined) {
    headers.set("Content-Type", "application/json");
  }

  if (options.csrfToken) {
    headers.set("X-CSRF-Token", options.csrfToken);
  }

  if (options.idempotencyKey) {
    headers.set("Idempotency-Key", options.idempotencyKey);
  }

  if (options.ifMatch) {
    headers.set("If-Match", options.ifMatch);
  }

  const response = await fetch(`/api/v1${path}`, {
    method: options.method ?? "GET",
    credentials: "include",
    cache: "no-store",
    headers,
    body: options.body !== undefined ? JSON.stringify(options.body) : undefined
  });

  const headerRequestId = response.headers.get("X-Request-ID");
  const etag = response.headers.get("ETag");

  if (!response.ok) {
    let errorMessage = `Request failed with status ${response.status}`;
    let errorCode = `HTTP_${response.status}`;
    let requestId = headerRequestId ?? undefined;

    try {
      const errorPayload = (await response.json()) as {
        error?: ApiErrorPayload;
        meta?: ApiMeta;
      };
      if (errorPayload?.meta?.requestId) {
        requestId = errorPayload.meta.requestId;
      }
      if (errorPayload?.error?.code) {
        errorCode = errorPayload.error.code;
      }
      if (errorPayload?.error?.message) {
        errorMessage = `${errorPayload.error.message} (${errorCode})`;
      }
    } catch {
      // Keep fallback status message when response body is not JSON.
    }

    throw new ApiClientError(errorMessage, response.status, errorCode, requestId);
  }

  if (response.status === 204) {
    return {
      data: undefined as T,
      meta: headerRequestId ? { requestId: headerRequestId } : undefined,
      etag,
      requestId: headerRequestId
    };
  }

  const payload = (await response.json()) as ApiResponse<T>;
  return {
    data: payload.data,
    meta: payload.meta,
    etag,
    requestId: payload.meta?.requestId ?? headerRequestId
  };
}

export async function apiRequest<T>(path: string, options: FetchOptions = {}): Promise<T> {
  const result = await apiRequestWithMeta<T>(path, options);
  return result.data;
}

export async function apiMutate<T>(
  path: string,
  options: Omit<FetchOptions, "csrfToken">
): Promise<{ data: T; meta?: ApiMeta; etag: string | null; requestId: string | null }> {
  const csrfToken = await getCsrfToken();
  return apiRequestWithMeta<T>(path, { ...options, csrfToken });
}

export async function probeHealth(): Promise<HealthProbeResult> {
  const checkedAt = new Date().toISOString();
  const [liveRes, readyRes] = await Promise.allSettled([
    fetch("/health/live", { cache: "no-store" }),
    fetch("/health/ready", { cache: "no-store" })
  ]);

  let live = { ok: false, status: 0, detail: "Unreachable" };
  if (liveRes.status === "fulfilled") {
    let detail = liveRes.value.ok ? "ok" : `HTTP ${liveRes.value.status}`;
    try {
      const body = (await liveRes.value.json()) as { status?: string };
      if (body?.status) detail = body.status;
    } catch {
      // ignore
    }
    live = { ok: liveRes.value.ok, status: liveRes.value.status, detail };
  }

  let ready: { ok: boolean; status: number; detail: string; code?: string } = {
    ok: false,
    status: 0,
    detail: "Unreachable"
  };
  if (readyRes.status === "fulfilled") {
    let detail = readyRes.value.ok ? "ready" : `HTTP ${readyRes.value.status}`;
    let code: string | undefined;
    try {
      const body = (await readyRes.value.json()) as { status?: string; code?: string };
      if (body?.status) detail = body.status;
      if (body?.code) code = body.code;
    } catch {
      // ignore
    }
    ready = { ok: readyRes.value.ok, status: readyRes.value.status, detail, code };
  }

  return { live, ready, checkedAt };
}
