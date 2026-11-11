"use client";

import React, { useCallback, useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { AppShell, useConsole } from "../../components/AppShell";
import { EmptyState, ErrorAlert, LoadingRows, SuccessAlert } from "../../components/StateBanner";
import {
  CloseIcon,
  EditIcon,
  GlobeIcon,
  PlusIcon,
  RefreshIcon,
  TrashIcon
} from "../../components/Icons";
import {
  apiMutate,
  apiRequest,
  apiRequestWithMeta,
  Policy,
  Upstream,
  Website,
  WebsiteSpec
} from "../../lib/api";
import { engineModeLabel } from "../../lib/format";

function WebsitesContent() {
  const { me, authLoading, refreshShell } = useConsole();

  const [loading, setLoading] = useState(true);
  const [websites, setWebsites] = useState<Website[]>([]);
  const [policies, setPolicies] = useState<Policy[]>([]);
  const [nextCursor, setNextCursor] = useState<string | null>(null);
  const [error, setError] = useState<unknown>(null);
  const [success, setSuccess] = useState("");

  const [search, setSearch] = useState("");
  const [statusFilter, setStatusFilter] = useState<"all" | "enabled" | "disabled">("all");

  const [drawerOpen, setDrawerOpen] = useState(false);
  const [editingSite, setEditingSite] = useState<Website | null>(null);
  const [editingEtag, setEditingEtag] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [formError, setFormError] = useState<unknown>(null);

  const [name, setName] = useState("");
  const [domainsText, setDomainsText] = useState("");
  const [enabled, setEnabled] = useState(true);
  const [policyId, setPolicyId] = useState("");
  const [upstreams, setUpstreams] = useState<Upstream[]>([{ scheme: "http", host: "", port: 8080 }]);
  const [serverName, setServerName] = useState("");
  const [connectSeconds, setConnectSeconds] = useState(5);
  const [readSeconds, setReadSeconds] = useState(60);

  const [deleteTarget, setDeleteTarget] = useState<Website | null>(null);
  const [deleting, setDeleting] = useState(false);

  const canWrite = me?.user.role === "admin" || me?.user.role === "operator";

  const loadData = useCallback(async (cursor?: string | null) => {
    setLoading(true);
    setError(null);
    try {
      const query = cursor ? `?limit=50&cursor=${encodeURIComponent(cursor)}` : "?limit=50";
      const [sitesRes, policyRes] = await Promise.all([
        apiRequestWithMeta<Website[]>(`/websites${query}`),
        apiRequest<Policy[]>("/policies?limit=200")
      ]);
      if (cursor) {
        setWebsites((prev) => [...prev, ...sitesRes.data]);
      } else {
        setWebsites(sitesRes.data);
      }
      setNextCursor(sitesRes.meta?.nextCursor ?? null);
      setPolicies(policyRes);
    } catch (err) {
      setError(err);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    if (!authLoading && me) {
      void loadData();
    } else if (!authLoading && !me) {
      setLoading(false);
    }
  }, [authLoading, me, loadData]);

  const policyMap = useMemo(() => {
    const map = new Map<string, Policy>();
    for (const p of policies) map.set(p.id, p);
    return map;
  }, [policies]);

  const detectionPolicies = useMemo(
    () => policies.filter((p) => p.engineMode === "detectionOnly"),
    [policies]
  );

  const filteredSites = useMemo(() => {
    const q = search.trim().toLowerCase();
    return websites.filter((site) => {
      if (statusFilter === "enabled" && !site.enabled) return false;
      if (statusFilter === "disabled" && site.enabled) return false;
      if (!q) return true;
      return (
        site.name.toLowerCase().includes(q) ||
        site.domains.some((d) => d.toLowerCase().includes(q)) ||
        site.upstreams.some((u) => u.host.toLowerCase().includes(q))
      );
    });
  }, [websites, search, statusFilter]);

  function openCreate() {
    setEditingSite(null);
    setEditingEtag(null);
    setFormError(null);
    setName("");
    setDomainsText("");
    setEnabled(true);
    setPolicyId(detectionPolicies[0]?.id ?? "");
    setUpstreams([{ scheme: "http", host: "10.24.20.50", port: 8080 }]);
    setServerName("");
    setConnectSeconds(5);
    setReadSeconds(60);
    setDrawerOpen(true);
  }

  async function openEdit(site: Website) {
    setFormError(null);
    setError(null);
    try {
      const fresh = await apiRequestWithMeta<Website>(`/websites/${site.id}`);
      const s = fresh.data;
      setEditingSite(s);
      setEditingEtag(fresh.etag ?? `"${s.id}-v${s.version}"`);
      setName(s.name);
      setDomainsText(s.domains.join(", "));
      setEnabled(s.enabled);
      setPolicyId(s.policyId);
      setUpstreams(s.upstreams.length > 0 ? s.upstreams.map((u) => ({ ...u })) : [{ scheme: "http", host: "", port: 8080 }]);
      setServerName(s.upstreamTls?.serverName ?? "");
      setConnectSeconds(s.timeouts?.connectSeconds ?? 5);
      setReadSeconds(s.timeouts?.readSeconds ?? 60);
      setDrawerOpen(true);
    } catch (err) {
      setError(err);
    }
  }

  async function handleSave(e: React.FormEvent) {
    e.preventDefault();
    setSaving(true);
    setFormError(null);
    setSuccess("");

    try {
      const domains = domainsText
        .split(/[\s,]+/)
        .map((d) => d.trim())
        .filter(Boolean);

      const payload: WebsiteSpec = {
        name: name.trim(),
        domains,
        enabled,
        listeners: {
          http: true,
          https: false,
          redirectHttpToHttps: false
        },
        certificateId: null,
        policyId,
        upstreams: upstreams.map((u) => ({
          scheme: u.scheme,
          host: u.host.trim(),
          port: Number(u.port)
        })),
        upstreamTls: {
          verify: true,
          serverName: serverName.trim(),
          trustSource: "system",
          trustBundleId: null
        },
        timeouts: {
          connectSeconds: Number(connectSeconds),
          readSeconds: Number(readSeconds)
        }
      };

      if (editingSite) {
        await apiMutate<Website>(`/websites/${editingSite.id}`, {
          method: "PATCH",
          ifMatch: editingEtag ?? `"${editingSite.id}-v${editingSite.version}"`,
          body: payload
        });
        setSuccess(`Website "${payload.name}" updated. Desired generation incremented.`);
      } else {
        await apiMutate<Website>("/websites", {
          method: "POST",
          body: payload
        });
        setSuccess(`Website "${payload.name}" created in desired configuration.`);
      }

      setDrawerOpen(false);
      await Promise.all([loadData(), refreshShell()]);
    } catch (err) {
      setFormError(err);
    } finally {
      setSaving(false);
    }
  }

  async function handleDelete() {
    if (!deleteTarget) return;
    setDeleting(true);
    setError(null);
    setSuccess("");
    try {
      const fresh = await apiRequestWithMeta<Website>(`/websites/${deleteTarget.id}`);
      const etag = fresh.etag ?? `"${fresh.data.id}-v${fresh.data.version}"`;
      await apiMutate(`/websites/${deleteTarget.id}`, {
        method: "DELETE",
        ifMatch: etag
      });
      setSuccess(`Website "${deleteTarget.name}" removed.`);
      setDeleteTarget(null);
      await Promise.all([loadData(), refreshShell()]);
    } catch (err) {
      setError(err);
      setDeleteTarget(null);
    } finally {
      setDeleting(false);
    }
  }

  function updateUpstream(index: number, patch: Partial<Upstream>) {
    setUpstreams((prev) => prev.map((u, i) => (i === index ? { ...u, ...patch } : u)));
  }

  return (
    <div className="stack-md">
      <div className="row-between">
        <div className="row" style={{ flex: 1, minWidth: 260 }}>
          <input
            className="input"
            style={{ maxWidth: 320 }}
            placeholder="Search by site name, domain, or upstream..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            aria-label="Search websites"
          />
          <div className="segmented">
            {(["all", "enabled", "disabled"] as const).map((tab) => (
              <button
                key={tab}
                type="button"
                className={`segmented-btn ${statusFilter === tab ? "active" : ""}`}
                onClick={() => setStatusFilter(tab)}
              >
                {tab === "all" ? "All" : tab === "enabled" ? "Enabled" : "Disabled"}
              </button>
            ))}
          </div>
        </div>

        <div className="row">
          <button type="button" onClick={() => void loadData()} className="btn btn-secondary btn-sm" disabled={loading}>
            <RefreshIcon size={14} />
            <span>Refresh</span>
          </button>
          {canWrite ? (
            <button type="button" onClick={openCreate} className="btn btn-primary btn-sm">
              <PlusIcon size={14} />
              <span>Add Website</span>
            </button>
          ) : null}
        </div>
      </div>

      {success ? <SuccessAlert message={success} onDismiss={() => setSuccess("")} /> : null}
      {error ? <ErrorAlert error={error} onRetry={() => void loadData()} /> : null}

      <section className="card card-flush">
        {loading && websites.length === 0 ? (
          <LoadingRows count={4} />
        ) : filteredSites.length === 0 ? (
          <EmptyState
            icon={<GlobeIcon size={20} />}
            title={websites.length === 0 ? "No websites configured yet" : "No websites match filters"}
            description={
              websites.length === 0
                ? "New websites require a DetectionOnly policy and an upstream host permitted in Proxy Allowed CIDRs."
                : "Adjust your search query or status filter."
            }
            action={
              websites.length === 0 && canWrite ? (
                <div className="row">
                  <button type="button" onClick={openCreate} className="btn btn-primary btn-sm">
                    <PlusIcon size={14} />
                    <span>Add Website</span>
                  </button>
                  <Link href="/settings" className="btn btn-secondary btn-sm">
                    Check Proxy CIDRs
                  </Link>
                </div>
              ) : null
            }
          />
        ) : (
          <div className="table-wrap">
            <table className="data-table">
              <thead>
                <tr>
                  <th>Website</th>
                  <th>Domains</th>
                  <th>Upstreams &amp; Timeouts</th>
                  <th>Assigned Policy</th>
                  <th>Listeners</th>
                  <th>State</th>
                  {canWrite ? <th style={{ textAlign: "right" }}>Actions</th> : null}
                </tr>
              </thead>
              <tbody>
                {filteredSites.map((site) => {
                  const policy = policyMap.get(site.policyId);
                  const mode = policy ? engineModeLabel(policy.engineMode) : null;
                  return (
                    <tr key={site.id}>
                      <td>
                        <div style={{ fontWeight: 650, color: "var(--ink)" }}>{site.name}</div>
                        <div className="small mono">v{site.version} · {site.id.slice(0, 12)}…</div>
                      </td>
                      <td>
                        <div className="row" style={{ gap: 4 }}>
                          {site.domains.map((d) => (
                            <span key={d} className="badge mono">
                              {d}
                            </span>
                          ))}
                        </div>
                      </td>
                      <td className="mono small">
                        {site.upstreams.map((u, idx) => (
                          <div key={idx}>
                            {u.scheme}://{u.host}:{u.port}
                          </div>
                        ))}
                        <div className="small">
                          Timeout: {site.timeouts?.connectSeconds ?? 5}s / {site.timeouts?.readSeconds ?? 60}s
                        </div>
                      </td>
                      <td>
                        <div style={{ fontWeight: 600, color: "var(--ink)" }}>
                          {policy?.name ?? site.policyId.slice(0, 12) + "…"}
                        </div>
                        {mode ? <span className={`badge ${mode.tone}`}>{mode.label}</span> : null}
                      </td>
                      <td>
                        <div className="row" style={{ gap: 4 }}>
                          {site.listeners?.http ? <span className="badge info">HTTP</span> : null}
                          <span className="badge" title="HTTPS listeners require Certificate Vault milestone">
                            HTTPS Pending
                          </span>
                        </div>
                      </td>
                      <td>
                        <span className={`badge ${site.enabled ? "brand" : "neutral"}`}>
                          {site.enabled ? "Enabled" : "Disabled"}
                        </span>
                      </td>
                      {canWrite ? (
                        <td style={{ textAlign: "right" }}>
                          <div className="row" style={{ justifyContent: "flex-end" }}>
                            <button
                              type="button"
                              className="btn btn-secondary btn-sm"
                              onClick={() => void openEdit(site)}
                            >
                              <EditIcon size={13} />
                              <span>Edit</span>
                            </button>
                            <button
                              type="button"
                              className="btn btn-danger btn-sm"
                              onClick={() => setDeleteTarget(site)}
                            >
                              <TrashIcon size={13} />
                              <span>Delete</span>
                            </button>
                          </div>
                        </td>
                      ) : null}
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}

        {nextCursor ? (
          <div style={{ padding: "0.9rem 1.2rem", borderTop: "1px solid var(--border)", textAlign: "center" }}>
            <button
              type="button"
              className="btn btn-secondary btn-sm"
              onClick={() => void loadData(nextCursor)}
              disabled={loading}
            >
              Load More Websites
            </button>
          </div>
        ) : null}
      </section>

      {/* Create / Edit Website Drawer */}
      {drawerOpen ? (
        <div className="backdrop" role="dialog" aria-modal="true" aria-labelledby="site-drawer-title">
          <div className="drawer">
            <div className="drawer-header">
              <div>
                <h2 id="site-drawer-title" style={{ fontSize: "1.1rem" }}>
                  {editingSite ? `Edit Website: ${editingSite.name}` : "Add Protected Website"}
                </h2>
                <p className="small">
                  {editingSite
                    ? `Optimistic lock ETag: ${editingEtag}`
                    : "Save desired virtual host routing, policy assignment, and SSRF-validated upstreams."}
                </p>
              </div>
              <button
                type="button"
                className="btn btn-ghost btn-icon"
                onClick={() => setDrawerOpen(false)}
                aria-label="Close drawer"
              >
                <CloseIcon />
              </button>
            </div>

            <form onSubmit={handleSave} className="drawer-body form-stack">
              {formError ? (
                <ErrorAlert
                  error={formError}
                  action={
                    <Link href="/settings" className="btn btn-secondary btn-sm">
                      Open Proxy Allowlist
                    </Link>
                  }
                />
              ) : null}

              {!editingSite && detectionPolicies.length === 0 ? (
                <div className="alert warning">
                  <div className="stack-sm">
                    <strong>DetectionOnly Policy Required</strong>
                    <p className="small">
                      New websites must initially reference a <code>DetectionOnly</code> policy. Create one on the{" "}
                      <Link href="/policies" style={{ textDecoration: "underline", fontWeight: 600 }}>
                        WAF Policies
                      </Link>{" "}
                      page first.
                    </p>
                  </div>
                </div>
              ) : null}

              <div className="field">
                <label className="field-label" htmlFor="site-name">
                  Website Name
                </label>
                <input
                  id="site-name"
                  className="input"
                  required
                  maxLength={256}
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  placeholder="e.g. ERP Production Portal"
                />
              </div>

              <div className="field">
                <label className="field-label" htmlFor="site-domains">
                  Domains (comma or newline separated)
                </label>
                <textarea
                  id="site-domains"
                  className="textarea mono"
                  rows={2}
                  required
                  value={domainsText}
                  onChange={(e) => setDomainsText(e.target.value)}
                  placeholder="erp.example.test, app.example.test"
                />
                <span className="field-hint">
                  Must be valid DNS hostnames containing a dot (no IP literals, wildcards, or localhost).
                </span>
              </div>

              <div className="field">
                <label className="field-label" htmlFor="site-policy">
                  Assigned Protection Policy
                </label>
                <select
                  id="site-policy"
                  className="select"
                  required
                  value={policyId}
                  onChange={(e) => setPolicyId(e.target.value)}
                >
                  <option value="" disabled>
                    Select a policy...
                  </option>
                  {(editingSite ? policies : detectionPolicies).map((p) => (
                    <option key={p.id} value={p.id}>
                      {p.name} ({engineModeLabel(p.engineMode).label})
                    </option>
                  ))}
                </select>
                {!editingSite ? (
                  <span className="field-hint">
                    Only <code>DetectionOnly</code> policies are eligible at initial site creation; you may switch to{" "}
                    <code>Blocking</code> after onboarding.
                  </span>
                ) : null}
              </div>

              <label className="checkbox-row">
                <input type="checkbox" checked={enabled} onChange={(e) => setEnabled(e.target.checked)} />
                <div>
                  <div style={{ fontWeight: 600 }}>Site Enabled</div>
                  <div className="small">Enabled websites enforce globally unique domain names.</div>
                </div>
              </label>

              <div className="divider" />
              <div className="row-between">
                <div>
                  <h3 style={{ fontSize: "0.95rem" }}>Upstreams (1–10)</h3>
                  <p className="small">
                    IPs/resolved hosts must belong to Admin Proxy Allowed CIDRs (<Link href="/settings" style={{ textDecoration: "underline" }}>Settings</Link>).
                  </p>
                </div>
                <button
                  type="button"
                  className="btn btn-secondary btn-sm"
                  disabled={upstreams.length >= 10}
                  onClick={() => setUpstreams([...upstreams, { scheme: "http", host: "", port: 8080 }])}
                >
                  <PlusIcon size={13} />
                  <span>Add Upstream</span>
                </button>
              </div>

              <div className="stack-sm">
                {upstreams.map((u, idx) => (
                  <div
                    key={idx}
                    className="card"
                    style={{ background: "var(--surface-2)", padding: "0.85rem", display: "grid", gridTemplateColumns: "100px 1fr 90px auto", gap: "0.5rem", alignItems: "end" }}
                  >
                    <div className="field">
                      <label className="field-label" htmlFor={`up-scheme-${idx}`}>
                        Scheme
                      </label>
                      <select
                        id={`up-scheme-${idx}`}
                        className="select"
                        value={u.scheme}
                        onChange={(e) => updateUpstream(idx, { scheme: e.target.value as "http" | "https" })}
                      >
                        <option value="http">http</option>
                        <option value="https">https</option>
                      </select>
                    </div>
                    <div className="field">
                      <label className="field-label" htmlFor={`up-host-${idx}`}>
                        Host / IP
                      </label>
                      <input
                        id={`up-host-${idx}`}
                        className="input mono"
                        required
                        value={u.host}
                        onChange={(e) => updateUpstream(idx, { host: e.target.value })}
                        placeholder="10.24.20.50"
                      />
                    </div>
                    <div className="field">
                      <label className="field-label" htmlFor={`up-port-${idx}`}>
                        Port
                      </label>
                      <input
                        id={`up-port-${idx}`}
                        className="input mono"
                        type="number"
                        min={1}
                        max={65535}
                        required
                        value={u.port}
                        onChange={(e) => updateUpstream(idx, { port: Number(e.target.value) })}
                      />
                    </div>
                    <button
                      type="button"
                      className="btn btn-danger btn-icon"
                      disabled={upstreams.length <= 1}
                      onClick={() => setUpstreams(upstreams.filter((_, i) => i !== idx))}
                      aria-label="Remove upstream"
                    >
                      <TrashIcon size={14} />
                    </button>
                  </div>
                ))}
              </div>

              <div className="grid-3">
                <div className="field">
                  <label className="field-label" htmlFor="tls-sni">
                    Upstream TLS SNI
                  </label>
                  <input
                    id="tls-sni"
                    className="input mono"
                    value={serverName}
                    onChange={(e) => setServerName(e.target.value)}
                    placeholder="optional hostname"
                  />
                </div>
                <div className="field">
                  <label className="field-label" htmlFor="connect-timeout">
                    Connect (1–30s)
                  </label>
                  <input
                    id="connect-timeout"
                    className="input mono"
                    type="number"
                    min={1}
                    max={30}
                    required
                    value={connectSeconds}
                    onChange={(e) => setConnectSeconds(Number(e.target.value))}
                  />
                </div>
                <div className="field">
                  <label className="field-label" htmlFor="read-timeout">
                    Read (1–300s)
                  </label>
                  <input
                    id="read-timeout"
                    className="input mono"
                    type="number"
                    min={1}
                    max={300}
                    required
                    value={readSeconds}
                    onChange={(e) => setReadSeconds(Number(e.target.value))}
                  />
                </div>
              </div>

              <div className="small">
                Listener mode is <code>HTTP</code>; upstream TLS verification is enforced using system trust until
                certificate storage is enabled.
              </div>

              <div className="drawer-footer" style={{ margin: "0 -1.35rem -1.25rem", marginTop: "0.5rem" }}>
                <button type="button" className="btn btn-secondary" onClick={() => setDrawerOpen(false)}>
                  Cancel
                </button>
                <button
                  type="submit"
                  className="btn btn-primary"
                  disabled={saving || !policyId}
                >
                  {saving ? "Saving..." : editingSite ? "Save Website Changes" : "Create Website"}
                </button>
              </div>
            </form>
          </div>
        </div>
      ) : null}

      {/* Delete Confirmation Modal */}
      {deleteTarget ? (
        <div className="backdrop backdrop-center" role="dialog" aria-modal="true" aria-labelledby="delete-site-title">
          <div className="modal">
            <div className="modal-header">
              <h2 id="delete-site-title" style={{ fontSize: "1.05rem" }}>
                Delete Website
              </h2>
              <button type="button" className="btn btn-ghost btn-icon" onClick={() => setDeleteTarget(null)}>
                <CloseIcon />
              </button>
            </div>
            <div className="modal-body stack-sm">
              <p>
                Remove website <strong>{deleteTarget.name}</strong> (
                <span className="mono">{deleteTarget.domains.join(", ")}</span>)?
              </p>
              <p className="small">This increments the desired configuration generation in SQLite.</p>
            </div>
            <div className="modal-footer">
              <button type="button" className="btn btn-secondary" onClick={() => setDeleteTarget(null)}>
                Cancel
              </button>
              <button type="button" className="btn btn-danger" onClick={() => void handleDelete()} disabled={deleting}>
                {deleting ? "Deleting..." : "Confirm Delete"}
              </button>
            </div>
          </div>
        </div>
      ) : null}
    </div>
  );
}

export default function WebsitesPage() {
  return (
    <AppShell
      title="Protected Websites"
      subtitle="Manage virtual hosts, domains, policy bindings, timeouts, and SSRF-validated upstreams."
    >
      <WebsitesContent />
    </AppShell>
  );
}
