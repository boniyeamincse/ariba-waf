"use client";

import React from "react";
import Link from "next/link";
import { describeApiError } from "../lib/format";
import { AlertTriangleIcon, CheckCircleIcon, LockIcon } from "./Icons";

export function ErrorAlert({
  error,
  onRetry,
  action
}: {
  error: unknown;
  onRetry?: () => void;
  action?: React.ReactNode;
}) {
  if (!error) return null;
  const info = describeApiError(error);
  const isAuth = info.code === "AUTH_REQUIRED" || info.code === "SESSION_EXPIRED";

  return (
    <div className={`alert ${isAuth ? "warning" : "danger"}`} role="alert">
      {isAuth ? (
        <LockIcon size={18} className="shrink-0" />
      ) : (
        <AlertTriangleIcon size={18} className="shrink-0" />
      )}
      <div style={{ flex: 1, minWidth: 0 }}>
        <div className="row" style={{ gap: "0.5rem" }}>
          <strong style={{ fontSize: "0.9rem" }}>{info.title}</strong>
          <span className="badge">{info.code}</span>
          {info.requestId ? <span className="small mono">req: {info.requestId}</span> : null}
        </div>
        <div className="row" style={{ marginTop: "0.5rem" }}>
          {isAuth ? (
            <Link href="/" className="btn btn-primary btn-sm">
              Sign in at Gateway
            </Link>
          ) : null}
          {onRetry ? (
            <button type="button" onClick={onRetry} className="btn btn-secondary btn-sm">
              Retry
            </button>
          ) : null}
          {action}
        </div>
      </div>
    </div>
  );
}

export function SuccessAlert({ message, onDismiss }: { message: string; onDismiss?: () => void }) {
  if (!message) return null;
  return (
    <div className="alert success" role="status">
      <CheckCircleIcon size={18} />
      <div style={{ flex: 1 }}>{message}</div>
      {onDismiss ? (
        <button type="button" className="btn btn-ghost btn-sm" onClick={onDismiss}>
          Dismiss
        </button>
      ) : null}
    </div>
  );
}

export function EmptyState({
  icon,
  title,
  description,
  action
}: {
  icon: React.ReactNode;
  title: string;
  description: string;
  action?: React.ReactNode;
}) {
  return (
    <div
      style={{
        padding: "2.5rem 1.5rem",
        textAlign: "center",
        display: "flex",
        flexDirection: "column",
        alignItems: "center",
        gap: "0.65rem"
      }}
    >
      <div
        style={{
          width: 44,
          height: 44,
          borderRadius: 12,
          background: "var(--surface-2)",
          border: "1px solid var(--border)",
          display: "grid",
          placeItems: "center",
          color: "var(--muted)"
        }}
      >
        {icon}
      </div>
      <h3 style={{ fontSize: "1rem" }}>{title}</h3>
      <p className="small" style={{ maxWidth: 460 }}>
        {description}
      </p>
      {action ? <div style={{ marginTop: "0.4rem" }}>{action}</div> : null}
    </div>
  );
}

export function LoadingRows({ count = 4 }: { count?: number }) {
  return (
    <div className="stack-sm" style={{ padding: "1rem 1.2rem" }} aria-busy="true" aria-label="Loading">
      {Array.from({ length: count }).map((_, idx) => (
        <div key={idx} className="skeleton" style={{ height: 42, width: "100%" }} />
      ))}
    </div>
  );
}
