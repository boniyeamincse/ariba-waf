"use client";

import React, { useCallback, useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { AppShell, useConsole } from "../../components/AppShell";
import { usePreferences } from "../../components/PreferencesContext";
import { EmptyState, ErrorAlert, LoadingRows } from "../../components/StateBanner";
import {
  ActivityIcon,
  GlobeIcon,
  PlusIcon,
  RefreshIcon,
  ServerIcon,
  ShieldIcon,
  SlidersIcon
} from "../../components/Icons";
import { apiRequest, Policy, Website } from "../../lib/api";
import { engineModeLabel, formatBytes, formatDateTime, roleBadgeMeta } from "../../lib/format";

function DashboardContent() {
  const { me, configStatus, health, authLoading, refreshShell } = useConsole();
  const { bilingual } = usePreferences();

  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<unknown>(null);
  const [websites, setWebsites] = useState<Website[]>([]);
  const [policies, setPolicies] = useState<Policy[]>([]);

  const loadOverview = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const [siteList, policyList] = await Promise.all([
        apiRequest<Website[]>("/websites?limit=50"),
        apiRequest<Policy[]>("/policies?limit=50")
      ]);
      setWebsites(siteList);
      setPolicies(policyList);
    } catch (err) {
      setError(err);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    if (!authLoading && me) {
      void loadOverview();
    } else if (!authLoading && !me) {
      setLoading(false);
    }
  }, [authLoading, me, loadOverview]);

  async function refreshAll() {
    await Promise.all([refreshShell(), loadOverview()]);
  }

  const policyMap = useMemo(() => {
    const map = new Map<string, Policy>();
    for (const p of policies) map.set(p.id, p);
    return map;
  }, [policies]);

  const enabledSites = websites.filter((w) => w.enabled).length;
  const blockingPolicies = policies.filter((p) => p.engineMode === "blocking").length;
  const detectPolicies = policies.filter((p) => p.engineMode === "detectionOnly").length;
  const roleInfo = me ? roleBadgeMeta(me.user.role) : null;
  const canEdit = me?.user.role === "admin" || me?.user.role === "operator";

  return (
    <div className="stack-md">
      <div className="row-between">
        <div className="row">
          <span className="badge info">Control Plane Telemetry</span>
          {bilingual ? <span className="small">ম্যানেজমেন্ট ওভারভিউ ও স্ট্যাটাস</span> : null}
        </div>
        <div className="row">
          <button type="button" onClick={() => void refreshAll()} className="btn btn-secondary btn-sm" disabled={loading}>
            <RefreshIcon size={14} />
            <span>Refresh Telemetry</span>
          </button>
          {canEdit ? (
            <Link href="/websites" className="btn btn-primary btn-sm">
              <PlusIcon size={14} />
              <span>Configure Website</span>
            </Link>
          ) : null}
        </div>
      </div>

      {error ? <ErrorAlert error={error} onRetry={loadOverview} /> : null}

      {/* KPI Command Strip */}
      <section className="kpi-grid" aria-label="Key Metrics">
        <article className="kpi-card">
          <div className="kpi-label">
            <span>Protected Websites</span>
            <GlobeIcon size={16} />
          </div>
          <div className="kpi-value">{loading ? "—" : websites.length}</div>
          <div className="kpi-meta">
            <span className="badge brand">{enabledSites} Enabled</span>{" "}
            <span className="small">{websites.length - enabledSites} Disabled</span>
          </div>
        </article>

        <article className="kpi-card">
          <div className="kpi-label">
            <span>WAF Policies</span>
            <SlidersIcon size={16} />
          </div>
          <div className="kpi-value">{loading ? "—" : policies.length}</div>
          <div className="kpi-meta">
            <span className="badge danger">{blockingPolicies} Blocking</span>{" "}
            <span className="badge warning">{detectPolicies} Detect</span>
          </div>
        </article>

        <article className="kpi-card">
          <div className="kpi-label">
            <span>Desired Generation</span>
            <ShieldIcon size={16} />
          </div>
          <div className="kpi-value mono">
            {configStatus ? `#${configStatus.desiredGeneration}` : "—"}
          </div>
          <div className="kpi-meta">
            {configStatus?.pendingChanges ? (
              <span className="badge warning">Pending Apply</span>
            ) : (
              <span className="badge brand">No Pending Drift</span>
            )}
          </div>
        </article>

        <article className="kpi-card">
          <div className="kpi-label">
            <span>Activation Executor</span>
            <ServerIcon size={16} />
          </div>
          <div className="kpi-value" style={{ fontSize: "1.2rem", paddingTop: 4 }}>
            {configStatus?.activationExecutorAvailable ? "Connected" : "Unavailable"}
          </div>
          <div className="kpi-meta">
            <span className={`badge ${health?.ready.ok ? "brand" : "warning"}`}>
              Readiness: {health?.ready.ok ? "200 Ready" : `${health?.ready.status || 503} Not Ready`}
            </span>
          </div>
        </article>
      </section>

      {/* Desired vs Applied Lifecycle Banner */}
      <section className="card">
        <div className="card-header">
          <div>
            <h2 className="card-title">Desired vs. Applied Configuration Lifecycle</h2>
            <p className="small">
              Writes to websites, policies, and proxy settings advance the SQLite desired generation atomically.
              {bilingual ? " (ডিজায়ার্ড স্টেট ও সক্রিয় রানিং রিভিশনের পার্থক্য)" : ""}
            </p>
          </div>
          <Link href="/settings" className="btn btn-secondary btn-sm">
            Inspect Lifecycle &amp; Proxy
          </Link>
        </div>

        <div className="grid-3">
          <div className="card" style={{ background: "var(--surface-2)", padding: "0.9rem" }}>
            <div className="small" style={{ fontWeight: 650 }}>
              1. Desired State (SQLite)
            </div>
            <div className="mono" style={{ fontSize: "1.1rem", fontWeight: 700, marginTop: 4 }}>
              Generation #{configStatus?.desiredGeneration ?? 0}
            </div>
            <p className="small" style={{ marginTop: 4 }}>
              Validated &amp; persisted with ETag concurrency.
            </p>
          </div>

          <div className="card" style={{ background: "var(--surface-2)", padding: "0.9rem" }}>
            <div className="small" style={{ fontWeight: 650 }}>
              2. Activation Executor
            </div>
            <div style={{ fontSize: "1.1rem", fontWeight: 700, marginTop: 4 }}>
              {configStatus?.activationExecutorAvailable ? "Online" : "Split-Dev Mode (Offline)"}
            </div>
            <p className="small" style={{ marginTop: 4 }}>
              {configStatus?.activationExecutorAvailable
                ? "Ready to render and reload proxy."
                : "Host Go process does not reload external Nginx; integrated container required."}
            </p>
          </div>

          <div className="card" style={{ background: "var(--surface-2)", padding: "0.9rem" }}>
            <div className="small" style={{ fontWeight: 650 }}>
              3. Applied Revision
            </div>
            <div className="mono" style={{ fontSize: "1.1rem", fontWeight: 700, marginTop: 4 }}>
              {configStatus?.appliedRevisionId ?? "None"}
            </div>
            <p className="small" style={{ marginTop: 4 }}>
              Active proxy revision currently serving traffic.
            </p>
          </div>
        </div>
      </section>

      {/* Websites + Policies Split */}
      <section className="grid-2">
        <article className="card card-flush">
          <div className="card-header-flush">
            <div>
              <h2 className="card-title">Configured Websites ({websites.length})</h2>
              <p className="small">Virtual hosts, listeners &amp; assigned WAF policies</p>
            </div>
            <Link href="/websites" className="btn btn-secondary btn-sm">
              Manage Websites
            </Link>
          </div>

          {loading ? (
            <LoadingRows count={3} />
          ) : websites.length === 0 ? (
            <EmptyState
              icon={<GlobeIcon size={20} />}
              title="No websites configured"
              description="Create a DetectionOnly policy first, ensure upstream CIDRs are allowed in Proxy Settings, then add a website."
              action={
                canEdit ? (
                  <Link href="/websites" className="btn btn-primary btn-sm">
                    Add First Website
                  </Link>
                ) : null
              }
            />
          ) : (
            <div className="table-wrap">
              <table className="data-table">
                <thead>
                  <tr>
                    <th>Site / Domains</th>
                    <th>Policy</th>
                    <th>Status</th>
                    <th>Ver</th>
                  </tr>
                </thead>
                <tbody>
                  {websites.slice(0, 6).map((site) => {
                    const policy = policyMap.get(site.policyId);
                    const mode = policy ? engineModeLabel(policy.engineMode) : null;
                    return (
                      <tr key={site.id}>
                        <td>
                          <div style={{ fontWeight: 650, color: "var(--ink)" }}>{site.name}</div>
                          <div className="small mono">{site.domains.join(", ")}</div>
                        </td>
                        <td>
                          <div style={{ fontSize: "0.84rem" }}>{policy?.name ?? site.policyId.slice(0, 10) + "…"}</div>
                          {mode ? <span className={`badge ${mode.tone}`}>{mode.label}</span> : null}
                        </td>
                        <td>
                          <span className={`badge ${site.enabled ? "brand" : "neutral"}`}>
                            {site.enabled ? "Enabled" : "Disabled"}
                          </span>
                        </td>
                        <td className="mono">v{site.version}</td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>
          )}
        </article>

        <article className="card card-flush">
          <div className="card-header-flush">
            <div>
              <h2 className="card-title">WAF Protection Policies ({policies.length})</h2>
              <p className="small">Coraza CRS paranoia levels, inspection &amp; rate limits</p>
            </div>
            <Link href="/policies" className="btn btn-secondary btn-sm">
              Manage Policies
            </Link>
          </div>

          {loading ? (
            <LoadingRows count={3} />
          ) : policies.length === 0 ? (
            <EmptyState
              icon={<SlidersIcon size={20} />}
              title="No WAF policies defined"
              description="Create a baseline DetectionOnly policy before onboarding your first protected website."
              action={
                canEdit ? (
                  <Link href="/policies" className="btn btn-primary btn-sm">
                    Create Policy
                  </Link>
                ) : null
              }
            />
          ) : (
            <div className="table-wrap">
              <table className="data-table">
                <thead>
                  <tr>
                    <th>Policy Name</th>
                    <th>Engine Mode</th>
                    <th>CRS / Body</th>
                    <th>Rate Limit</th>
                  </tr>
                </thead>
                <tbody>
                  {policies.slice(0, 6).map((policy) => {
                    const mode = engineModeLabel(policy.engineMode);
                    return (
                      <tr key={policy.id}>
                        <td>
                          <div style={{ fontWeight: 650, color: "var(--ink)" }}>{policy.name}</div>
                          <div className="small mono">v{policy.version}</div>
                        </td>
                        <td>
                          <span className={`badge ${mode.tone}`}>{mode.label}</span>
                        </td>
                        <td className="small mono">
                          PL{policy.crs.paranoiaLevel} · {formatBytes(policy.inspection.requestBodyLimitBytes)}
                        </td>
                        <td className="small">
                          {policy.rateLimit.enabled
                            ? `${policy.rateLimit.requestsPerSecond}/s (b:${policy.rateLimit.burst})`
                            : "Disabled"}
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>
          )}
        </article>
      </section>

      {/* Identity & Security Telemetry */}
      <section className="grid-2">
        <article className="card stack-sm">
          <div className="card-header" style={{ marginBottom: 0 }}>
            <h2 className="card-title">Operator Session &amp; RBAC Scope</h2>
            {roleInfo ? <span className={`badge ${roleInfo.tone}`}>{roleInfo.label}</span> : null}
          </div>
          {me ? (
            <div className="stack-sm">
              <div className="row-between small">
                <span>Authenticated Account</span>
                <strong style={{ color: "var(--ink)" }}>
                  {me.user.displayName} (<code>{me.user.username}</code>)
                </strong>
              </div>
              <div className="row-between small">
                <span>Session Expiry</span>
                <span className="mono">{formatDateTime(me.expiresAt)}</span>
              </div>
              <div className="row-between small">
                <span>Write Permissions</span>
                <span>
                  {me.user.role === "admin"
                    ? "Full Admin (Users, Proxy, Websites, Policies)"
                    : me.user.role === "operator"
                      ? "Operator (Websites & Policies)"
                      : "Viewer (Read-Only)"}
                </span>
              </div>
              <div className="row" style={{ marginTop: 6 }}>
                <Link href="/account" className="btn btn-secondary btn-sm">
                  Manage Sessions &amp; Password
                </Link>
                {me.user.role === "admin" ? (
                  <Link href="/users" className="btn btn-secondary btn-sm">
                    Manage Users
                  </Link>
                ) : null}
              </div>
            </div>
          ) : (
            <p className="small">Session details unavailable.</p>
          )}
        </article>

        <article className="card stack-sm">
          <div className="card-header" style={{ marginBottom: 0 }}>
            <h2 className="card-title">Security Events &amp; Telemetry Pipeline</h2>
            <span className="badge warning">Collector Pending</span>
          </div>
          <p className="small">
            Rule match vs. confirmed block telemetry requires an integrated event collector. Do not confuse an
            unconnected collector with zero attacks.
          </p>
          <div className="row" style={{ marginTop: 6 }}>
            <Link href="/events" className="btn btn-secondary btn-sm">
              <ActivityIcon size={14} />
              <span>Open Events Console</span>
            </Link>
          </div>
        </article>
      </section>
    </div>
  );
}

export default function DashboardPage() {
  return (
    <AppShell
      title="Operations Overview"
      subtitle="Live posture, desired configuration generation, websites, and protection policies."
    >
      <DashboardContent />
    </AppShell>
  );
}
