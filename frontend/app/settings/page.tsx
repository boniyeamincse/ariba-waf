"use client";

import React, { useCallback, useEffect, useState } from "react";
import { AppShell, useConsole } from "../../components/AppShell";
import { ErrorAlert, LoadingRows, SuccessAlert } from "../../components/StateBanner";
import {
  CloseIcon,
  LockIcon,
  PlusIcon,
  RefreshIcon,
  ServerIcon,
  ShieldIcon
} from "../../components/Icons";
import { ApiClientError, apiMutate, apiRequestWithMeta, ProxySettings } from "../../lib/api";
import { formatDateTime } from "../../lib/format";

function CidrListEditor({
  label,
  hint,
  cidrs,
  onChange,
  disabled,
  placeholder
}: {
  label: string;
  hint: string;
  cidrs: string[];
  onChange: (next: string[]) => void;
  disabled: boolean;
  placeholder: string;
}) {
  const [draft, setDraft] = useState("");

  function addCidr() {
    const val = draft.trim();
    if (!val) return;
    if (!cidrs.includes(val)) {
      onChange([...cidrs, val]);
    }
    setDraft("");
  }

  return (
    <div className="field">
      <div className="field-label">
        <span>{label}</span>
        <span className="small mono">{cidrs.length} / 64 CIDRs</span>
      </div>
      <p className="field-hint">{hint}</p>

      <div className="row" style={{ marginTop: 4 }}>
        <input
          className="input mono"
          style={{ flex: 1, minWidth: 200 }}
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter") {
              e.preventDefault();
              addCidr();
            }
          }}
          placeholder={placeholder}
          disabled={disabled || cidrs.length >= 64}
        />
        <button
          type="button"
          className="btn btn-secondary"
          onClick={addCidr}
          disabled={disabled || !draft.trim() || cidrs.length >= 64}
        >
          <PlusIcon size={14} />
          <span>Add CIDR</span>
        </button>
      </div>

      {cidrs.length === 0 ? (
        <div className="small" style={{ padding: "0.55rem 0.75rem", background: "var(--surface-2)", borderRadius: 8, border: "1px dashed var(--border-strong)" }}>
          Empty list (no CIDRs configured).
        </div>
      ) : (
        <div className="row" style={{ gap: 6, marginTop: 4 }}>
          {cidrs.map((cidr) => (
            <span key={cidr} className="badge info mono" style={{ paddingRight: disabled ? undefined : 6 }}>
              <span>{cidr}</span>
              {!disabled ? (
                <button
                  type="button"
                  onClick={() => onChange(cidrs.filter((c) => c !== cidr))}
                  aria-label={`Remove ${cidr}`}
                  style={{
                    background: "transparent",
                    border: 0,
                    color: "inherit",
                    cursor: "pointer",
                    display: "grid",
                    placeItems: "center",
                    padding: 2
                  }}
                >
                  <CloseIcon size={12} />
                </button>
              ) : null}
            </span>
          ))}
        </div>
      )}
    </div>
  );
}

function SettingsContent() {
  const { me, configStatus, health, authLoading, refreshShell } = useConsole();

  const [loading, setLoading] = useState(true);
  const [proxy, setProxy] = useState<ProxySettings | null>(null);
  const [etag, setEtag] = useState<string | null>(null);
  const [allowedCidrs, setAllowedCidrs] = useState<string[]>([]);
  const [trustedProxyCidrs, setTrustedProxyCidrs] = useState<string[]>([]);
  const [forbidden, setForbidden] = useState(false);

  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<unknown>(null);
  const [success, setSuccess] = useState("");

  const isAdmin = me?.user.role === "admin";

  const loadProxySettings = useCallback(async () => {
    setLoading(true);
    setError(null);
    setForbidden(false);
    try {
      const res = await apiRequestWithMeta<ProxySettings>("/settings/proxy");
      setProxy(res.data);
      setEtag(res.etag ?? `"proxy-v${res.data.version}"`);
      setAllowedCidrs(res.data.allowedCidrs);
      setTrustedProxyCidrs(res.data.trustedProxyCidrs);
    } catch (err) {
      if (err instanceof ApiClientError && (err.status === 403 || err.code === "FORBIDDEN")) {
        setForbidden(true);
      } else {
        setError(err);
      }
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    if (!authLoading && me) {
      if (isAdmin) {
        void loadProxySettings();
      } else {
        setForbidden(true);
        setLoading(false);
      }
    } else if (!authLoading && !me) {
      setLoading(false);
    }
  }, [authLoading, me, isAdmin, loadProxySettings]);

  async function saveProxySettings(e: React.FormEvent) {
    e.preventDefault();
    if (!proxy) return;
    setSaving(true);
    setError(null);
    setSuccess("");
    try {
      const res = await apiMutate<ProxySettings>("/settings/proxy", {
        method: "PATCH",
        ifMatch: etag ?? `"proxy-v${proxy.version}"`,
        body: {
          allowedCidrs,
          trustedProxyCidrs
        }
      });
      setProxy(res.data);
      setEtag(res.etag ?? `"proxy-v${res.data.version}"`);
      setAllowedCidrs(res.data.allowedCidrs);
      setTrustedProxyCidrs(res.data.trustedProxyCidrs);
      setSuccess(`Proxy network policy saved (v${res.data.version}). Desired generation advanced to #${res.data.desiredGeneration}.`);
      await refreshShell();
    } catch (err) {
      setError(err);
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="stack-md">
      {success ? <SuccessAlert message={success} onDismiss={() => setSuccess("")} /> : null}
      {error ? <ErrorAlert error={error} onRetry={() => void loadProxySettings()} /> : null}

      {/* Configuration & Activation State */}
      <section className="card stack-md">
        <div className="row-between">
          <div>
            <h2 className="card-title">Configuration Lifecycle &amp; Runtime Probes</h2>
            <p className="small">
              Tracks desired SQLite generation versus active proxy revision and process health endpoints.
            </p>
          </div>
          <button type="button" className="btn btn-secondary btn-sm" onClick={() => void refreshShell()}>
            <RefreshIcon size={14} />
            <span>Re-probe Status</span>
          </button>
        </div>

        <div className="grid-3">
          <div className="card" style={{ background: "var(--surface-2)", padding: "0.95rem" }}>
            <div className="small">Desired Generation</div>
            <div className="mono" style={{ fontSize: "1.35rem", fontWeight: 700, marginTop: 4 }}>
              #{configStatus?.desiredGeneration ?? 0}
            </div>
            <div style={{ marginTop: 6 }}>
              <span className={`badge ${configStatus?.pendingChanges ? "warning" : "brand"}`}>
                {configStatus?.pendingChanges ? "Pending Activation" : "In Sync"}
              </span>
            </div>
          </div>

          <div className="card" style={{ background: "var(--surface-2)", padding: "0.95rem" }}>
            <div className="small">Applied Proxy Revision</div>
            <div className="mono" style={{ fontSize: "1.35rem", fontWeight: 700, marginTop: 4 }}>
              {configStatus?.appliedRevisionId ?? "None"}
            </div>
            <div className="small" style={{ marginTop: 6 }}>
              Executor:{" "}
              <strong>{configStatus?.activationExecutorAvailable ? "Available" : "Unavailable (Split Dev)"}</strong>
            </div>
          </div>

          <div className="card" style={{ background: "var(--surface-2)", padding: "0.95rem" }}>
            <div className="small">Health Probes</div>
            <div className="stack-sm" style={{ marginTop: 6 }}>
              <div className="row-between small">
                <span>/health/live</span>
                <span className={`badge ${health?.live.ok ? "brand" : "danger"}`}>
                  {health?.live.status ?? 0} {health?.live.detail}
                </span>
              </div>
              <div className="row-between small">
                <span>/health/ready</span>
                <span className={`badge ${health?.ready.ok ? "brand" : "warning"}`}>
                  {health?.ready.status ?? 0} {health?.ready.code ?? health?.ready.detail}
                </span>
              </div>
              {health?.checkedAt ? (
                <div className="small mono" style={{ fontSize: "0.72rem" }}>
                  Checked: {formatDateTime(health.checkedAt)}
                </div>
              ) : null}
            </div>
          </div>
        </div>

        <div className="alert info">
          <ServerIcon size={18} />
          <div className="row-between" style={{ flex: 1 }}>
            <div className="small" style={{ color: "var(--ink-secondary)" }}>
              <strong style={{ color: "var(--ink)" }}>Activation Guardrail:</strong>{" "}
              {configStatus?.activationExecutorAvailable
                ? "Activation executor is online."
                : "Apply / Rollback jobs are disabled in split-process local development because the Go API cannot reload an external Nginx container."}
            </div>
            <button
              type="button"
              className="btn btn-primary btn-sm"
              disabled={!configStatus?.activationExecutorAvailable}
              title="Activation executor unavailable in current runtime"
            >
              Apply Generation #{configStatus?.desiredGeneration ?? 0}
            </button>
          </div>
        </div>
      </section>

      {/* Proxy CIDR Settings */}
      <section className="card stack-md">
        <div className="row-between">
          <div>
            <h2 className="card-title">Proxy Network &amp; SSRF Allowlists</h2>
            <p className="small">
              Admin-only egress destination allowlists and trusted proxy CIDRs.
              {proxy ? ` Current version: v${proxy.version}` : ""}
            </p>
          </div>
          {etag ? <span className="badge mono">ETag {etag}</span> : null}
        </div>

        {forbidden ? (
          <div className="alert warning">
            <LockIcon size={18} />
            <div className="small">
              <strong>Administrator Role Required:</strong> Reading or modifying proxy CIDR settings is restricted to{" "}
              <code>admin</code> accounts.
            </div>
          </div>
        ) : loading ? (
          <LoadingRows count={3} />
        ) : (
          <form onSubmit={saveProxySettings} className="form-stack">
            <div className="alert info">
              <ShieldIcon size={18} />
              <div className="small" style={{ color: "var(--ink-secondary)" }}>
                Loopback, link-local, multicast, metadata/service IPs, and management ports (<code>9000</code>,{" "}
                <code>9443</code>) are always blocked even with broad allowlists. Removing a CIDR used by an existing
                website upstream is rejected with <code>409 UPSTREAM_POLICY_CONFLICT</code>.
              </div>
            </div>

            <CidrListEditor
              label="Allowed Upstream CIDRs (allowedCidrs)"
              hint="Only upstream IP literals and DNS answers matching these networks may be saved on Websites."
              cidrs={allowedCidrs}
              onChange={setAllowedCidrs}
              disabled={!isAdmin || saving}
              placeholder="e.g. 10.24.20.0/24 or 192.168.10.0/24"
            />

            <div className="divider" />

            <CidrListEditor
              label="Trusted Reverse Proxy CIDRs (trustedProxyCidrs)"
              hint="Stored as desired proxy configuration for forwarding-header extraction in the integrated WAF."
              cidrs={trustedProxyCidrs}
              onChange={setTrustedProxyCidrs}
              disabled={!isAdmin || saving}
              placeholder="e.g. 10.0.0.0/8"
            />

            <div className="row" style={{ justifyContent: "flex-end", marginTop: 4 }}>
              <button type="button" className="btn btn-secondary" onClick={() => void loadProxySettings()} disabled={saving}>
                Reset to Saved
              </button>
              <button type="submit" className="btn btn-primary" disabled={saving || !proxy}>
                {saving ? "Saving Policy..." : "Save Proxy Settings"}
              </button>
            </div>
          </form>
        )}
      </section>
    </div>
  );
}

export default function SettingsPage() {
  return (
    <AppShell
      title="Configuration & Proxy"
      subtitle="Desired vs. applied configuration lifecycle, health probes, and SSRF upstream CIDR allowlists."
    >
      <SettingsContent />
    </AppShell>
  );
}
