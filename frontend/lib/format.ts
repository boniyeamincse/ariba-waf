import { ApiClientError, EngineMode, Role } from "./api";

export function formatBytes(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes < 0) return "0 B";
  if (bytes >= 1024 * 1024) {
    const mib = bytes / (1024 * 1024);
    return `${Number.isInteger(mib) ? mib : mib.toFixed(1)} MiB`;
  }
  if (bytes >= 1024) {
    const kib = bytes / 1024;
    return `${Number.isInteger(kib) ? kib : kib.toFixed(1)} KiB`;
  }
  return `${bytes} B`;
}

export function formatDateTime(value: string | null | undefined): string {
  if (!value) return "—";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleString(undefined, {
    year: "numeric",
    month: "short",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit"
  });
}

export function formatTimeRemaining(expiresAt: string | null | undefined): string {
  if (!expiresAt) return "—";
  const diffMs = new Date(expiresAt).getTime() - Date.now();
  if (Number.isNaN(diffMs)) return "—";
  if (diffMs <= 0) return "Expired";
  const mins = Math.floor(diffMs / 60000);
  const hrs = Math.floor(mins / 60);
  const remMins = mins % 60;
  if (hrs > 0) return `${hrs}h ${remMins}m`;
  return `${mins}m`;
}

const ERROR_HINTS: Record<string, string> = {
  AUTH_REQUIRED: "Authentication session missing or expired. Please sign in again.",
  SESSION_EXPIRED: "Your management session expired due to inactivity or age. Sign in again.",
  REAUTH_REQUIRED: "This sensitive action requires recent re-authentication (Account Security).",
  FORBIDDEN: "Your current role does not permit this operation.",
  ORIGIN_NOT_ALLOWED: "Request Origin rejected by management API origin policy.",
  CSRF_INVALID: "CSRF token verification failed. Refresh and retry.",
  RATE_LIMITED: "Too many requests or failed credential attempts. Wait before retrying.",
  INVALID_CREDENTIALS: "Invalid username, password, or enrollment token.",
  SETUP_COMPLETE: "Initial admin enrollment has already been completed.",
  IDEMPOTENCY_CONFLICT: "Idempotency key was reused with different request parameters.",
  VALIDATION_FAILED: "One or more fields failed server validation. Check ranges and formats.",
  UNKNOWN_FIELD: "Request contained a field rejected by strict schema validation.",
  VERSION_CONFLICT: "Resource was modified elsewhere (stale ETag). Reload latest version and retry.",
  PRECONDITION_REQUIRED: "Missing ETag precondition (If-Match). Reload resource before updating.",
  DOMAIN_CONFLICT: "One or more domains conflict with another enabled website.",
  POLICY_NOT_FOUND: "Selected protection policy no longer exists.",
  INITIAL_POLICY_MUST_DETECT: "New websites must initially use a DetectionOnly policy before switching modes.",
  CERTIFICATES_UNAVAILABLE: "HTTPS site listeners and certificate IDs are unavailable until certificate storage is enabled.",
  TRUST_BUNDLES_UNAVAILABLE: "Custom upstream trust bundles are unavailable; use system trust.",
  UPSTREAM_TLS_VERIFICATION_REQUIRED: "Upstream TLS certificate verification cannot be disabled.",
  UPSTREAM_NOT_ALLOWED: "Upstream host/IP is blocked by SSRF policy or missing from Proxy Allowed CIDRs.",
  UPSTREAM_CIDR_DENIED: "Upstream IP/resolved address is not in Proxy Allowed CIDRs (see Settings).",
  UPSTREAM_PORT_RESERVED: "Ports 9000 and 9443 are reserved for the management plane.",
  UPSTREAM_POLICY_CONFLICT: "New CIDR allowlist would invalidate an existing website upstream.",
  UNSUPPORTED_INSPECTION_SETTING: "Response body inspection is not supported in this milestone.",
  RESOURCE_IN_USE: "Policy is referenced by one or more websites and cannot be deleted.",
  USERNAME_CONFLICT: "A user with this username already exists.",
  LAST_ADMIN: "Cannot demote, disable, or delete the last enabled administrator.",
  STORAGE_UNAVAILABLE: "Management database is temporarily unavailable."
};

export function describeApiError(err: unknown): {
  title: string;
  code: string;
  requestId?: string;
  status?: number;
} {
  if (err instanceof ApiClientError) {
    const hint = ERROR_HINTS[err.code];
    return {
      title: hint ?? err.message,
      code: err.code,
      requestId: err.requestId,
      status: err.status
    };
  }
  if (err instanceof Error) {
    return {
      title: err.message,
      code: "CLIENT_ERROR"
    };
  }
  return {
    title: "An unexpected error occurred.",
    code: "UNKNOWN_ERROR"
  };
}

export function engineModeLabel(mode: EngineMode | string): {
  label: string;
  bn: string;
  tone: "success" | "warning" | "neutral" | "danger";
} {
  switch (mode) {
    case "blocking":
      return { label: "Blocking", bn: "ব্লকিং (Enforce)", tone: "danger" };
    case "detectionOnly":
      return { label: "DetectionOnly", bn: "ডিটেকশন (Monitor)", tone: "warning" };
    case "off":
      return { label: "Off", bn: "অফ (Bypass)", tone: "neutral" };
    default:
      return { label: mode, bn: mode, tone: "neutral" };
  }
}

export function roleBadgeMeta(role: Role | string): {
  label: string;
  tone: "brand" | "info" | "neutral";
} {
  switch (role) {
    case "admin":
      return { label: "Admin", tone: "brand" };
    case "operator":
      return { label: "Operator", tone: "info" };
    default:
      return { label: "Viewer", tone: "neutral" };
  }
}
