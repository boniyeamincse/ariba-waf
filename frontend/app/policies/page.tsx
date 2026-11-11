"use client";

import React, { useCallback, useEffect, useState } from "react";
import { AppShell, useConsole } from "../../components/AppShell";
import { usePreferences } from "../../components/PreferencesContext";
import { EmptyState, ErrorAlert, LoadingRows, SuccessAlert } from "../../components/StateBanner";
import {
  CloseIcon,
  EditIcon,
  PlusIcon,
  RefreshIcon,
  ShieldIcon,
  SlidersIcon,
  TrashIcon
} from "../../components/Icons";
import {
  apiMutate,
  apiRequest,
  apiRequestWithMeta,
  EngineMode,
  OversizeAction,
  ParserErrorAction,
  Policy,
  PolicySpec
} from "../../lib/api";
import { engineModeLabel, formatBytes } from "../../lib/format";

const DEFAULT_POLICY_FORM: PolicySpec = {
  name: "",
  engineMode: "detectionOnly",
  crs: {
    paranoiaLevel: 1,
    inboundAnomalyThreshold: 5,
    outboundAnomalyThreshold: 4
  },
  inspection: {
    requestBodyEnabled: true,
    requestBodyLimitBytes: 10 * 1024 * 1024,
    requestBodyNoFilesLimitBytes: 1 * 1024 * 1024,
    oversizeAction: "reject",
    parserErrorAction: "reject",
    responseBodyEnabled: false
  },
  rateLimit: {
    enabled: false,
    requestsPerSecond: 10,
    burst: 20
  }
};

function PoliciesContent() {
  const { me, authLoading, refreshShell } = useConsole();
  const { bilingual } = usePreferences();

  const [loading, setLoading] = useState(true);
  const [policies, setPolicies] = useState<Policy[]>([]);
  const [nextCursor, setNextCursor] = useState<string | null>(null);
  const [error, setError] = useState<unknown>(null);
  const [success, setSuccess] = useState("");

  const [drawerOpen, setDrawerOpen] = useState(false);
  const [editingPolicy, setEditingPolicy] = useState<Policy | null>(null);
  const [editingEtag, setEditingEtag] = useState<string | null>(null);
  const [form, setForm] = useState<PolicySpec>(DEFAULT_POLICY_FORM);
  const [saving, setSaving] = useState(false);
  const [formError, setFormError] = useState<unknown>(null);

  const [deleteTarget, setDeleteTarget] = useState<Policy | null>(null);
  const [deleting, setDeleting] = useState(false);

  const canWrite = me?.user.role === "admin" || me?.user.role === "operator";

  const loadPolicies = useCallback(async (cursor?: string | null) => {
    setLoading(true);
    setError(null);
    try {
      const query = cursor ? `?limit=50&cursor=${encodeURIComponent(cursor)}` : "?limit=50";
      const res = await apiRequestWithMeta<Policy[]>(`/policies${query}`);
      if (cursor) {
        setPolicies((prev) => [...prev, ...res.data]);
      } else {
        setPolicies(res.data);
      }
      setNextCursor(res.meta?.nextCursor ?? null);
    } catch (err) {
      setError(err);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    if (!authLoading && me) {
      void loadPolicies();
    } else if (!authLoading && !me) {
      setLoading(false);
    }
  }, [authLoading, me, loadPolicies]);

  function openCreate() {
    setEditingPolicy(null);
    setEditingEtag(null);
    setForm(JSON.parse(JSON.stringify(DEFAULT_POLICY_FORM)) as PolicySpec);
    setFormError(null);
    setDrawerOpen(true);
  }

  async function openEdit(policy: Policy) {
    setFormError(null);
    setError(null);
    try {
      const fresh = await apiRequestWithMeta<Policy>(`/policies/${policy.id}`);
      const p = fresh.data;
      setEditingPolicy(p);
      setEditingEtag(fresh.etag ?? `"${p.id}-v${p.version}"`);
      setForm({
        name: p.name,
        engineMode: p.engineMode,
        crs: { ...p.crs },
        inspection: { ...p.inspection, responseBodyEnabled: false },
        rateLimit: { ...p.rateLimit }
      });
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
      const payload: PolicySpec = {
        ...form,
        name: form.name.trim(),
        inspection: {
          ...form.inspection,
          responseBodyEnabled: false
        }
      };

      if (editingPolicy) {
        await apiMutate<Policy>(`/policies/${editingPolicy.id}`, {
          method: "PATCH",
          ifMatch: editingEtag ?? `"${editingPolicy.id}-v${editingPolicy.version}"`,
          body: payload
        });
        setSuccess(`Policy "${payload.name}" updated. Desired configuration generation incremented.`);
      } else {
        await apiMutate<Policy>("/policies", {
          method: "POST",
          body: payload
        });
        setSuccess(`Policy "${payload.name}" created.`);
      }

      setDrawerOpen(false);
      await Promise.all([loadPolicies(), refreshShell()]);
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
      const fresh = await apiRequestWithMeta<Policy>(`/policies/${deleteTarget.id}`);
      const etag = fresh.etag ?? `"${fresh.data.id}-v${fresh.data.version}"`;
      await apiMutate(`/policies/${deleteTarget.id}`, {
        method: "DELETE",
        ifMatch: etag
      });
      setSuccess(`Policy "${deleteTarget.name}" deleted.`);
      setDeleteTarget(null);
      await Promise.all([loadPolicies(), refreshShell()]);
    } catch (err) {
      setError(err);
      setDeleteTarget(null);
    } finally {
      setDeleting(false);
    }
  }

  return (
    <div className="stack-md">
      <div className="alert info">
        <ShieldIcon size={18} />
        <div className="small" style={{ color: "var(--ink-secondary)" }}>
          <strong style={{ color: "var(--ink)" }}>DetectionOnly vs. Blocking:</strong>{" "}
          <code>DetectionOnly</code> logs OWASP CRS rule matches without blocking requests, though transport &amp;
          oversize/parser limits (when set to <code>reject</code>) can still reject malformed payloads. New websites
          must initially attach a <code>DetectionOnly</code> policy before switching to <code>Blocking</code>.
          {bilingual ? " (DetectionOnly মোডে রুল ম্যাচ লগ হয়, তবে ওভারসাইজ বডি রিজেক্ট হতে পারে।)" : ""}
        </div>
      </div>

      <div className="row-between">
        <span className="small">
          Showing <strong>{policies.length}</strong> protection {policies.length === 1 ? "policy" : "policies"}
        </span>
        <div className="row">
          <button type="button" onClick={() => void loadPolicies()} className="btn btn-secondary btn-sm" disabled={loading}>
            <RefreshIcon size={14} />
            <span>Refresh</span>
          </button>
          {canWrite ? (
            <button type="button" onClick={openCreate} className="btn btn-primary btn-sm">
              <PlusIcon size={14} />
              <span>Create Policy</span>
            </button>
          ) : null}
        </div>
      </div>

      {success ? <SuccessAlert message={success} onDismiss={() => setSuccess("")} /> : null}
      {error ? <ErrorAlert error={error} onRetry={() => void loadPolicies()} /> : null}

      <section className="card card-flush">
        {loading && policies.length === 0 ? (
          <LoadingRows count={4} />
        ) : policies.length === 0 ? (
          <EmptyState
            icon={<SlidersIcon size={20} />}
            title="No WAF policies configured"
            description="Create a baseline DetectionOnly policy to configure CRS paranoia levels, body inspection limits, and rate limiting."
            action={
              canWrite ? (
                <button type="button" onClick={openCreate} className="btn btn-primary btn-sm">
                  <PlusIcon size={14} />
                  <span>Create Baseline Policy</span>
                </button>
              ) : null
            }
          />
        ) : (
          <div className="table-wrap">
            <table className="data-table">
              <thead>
                <tr>
                  <th>Policy</th>
                  <th>Engine Mode</th>
                  <th>CRS Tuning</th>
                  <th>Body Inspection</th>
                  <th>Rate Limit</th>
                  <th>Version</th>
                  {canWrite ? <th style={{ textAlign: "right" }}>Actions</th> : null}
                </tr>
              </thead>
              <tbody>
                {policies.map((policy) => {
                  const mode = engineModeLabel(policy.engineMode);
                  return (
                    <tr key={policy.id}>
                      <td>
                        <div style={{ fontWeight: 650, color: "var(--ink)" }}>{policy.name}</div>
                        <div className="small mono">{policy.id.slice(0, 14)}…</div>
                      </td>
                      <td>
                        <span className={`badge ${mode.tone}`}>{mode.label}</span>
                        {bilingual ? <div className="small" style={{ marginTop: 3 }}>{mode.bn}</div> : null}
                      </td>
                      <td className="mono small">
                        <div>Paranoia: PL{policy.crs.paranoiaLevel}</div>
                        <div>
                          Thresholds: In {policy.crs.inboundAnomalyThreshold} / Out{" "}
                          {policy.crs.outboundAnomalyThreshold}
                        </div>
                      </td>
                      <td className="small">
                        <div>
                          {policy.inspection.requestBodyEnabled ? (
                            <span className="mono">
                              {formatBytes(policy.inspection.requestBodyLimitBytes)} (no-files:{" "}
                              {formatBytes(policy.inspection.requestBodyNoFilesLimitBytes)})
                            </span>
                          ) : (
                            "Body inspection off"
                          )}
                        </div>
                        <div className="small">
                          Oversize: <code>{policy.inspection.oversizeAction}</code> · Parser:{" "}
                          <code>{policy.inspection.parserErrorAction}</code>
                        </div>
                      </td>
                      <td className="small">
                        {policy.rateLimit.enabled ? (
                          <span className="badge info mono">
                            {policy.rateLimit.requestsPerSecond} req/s (burst {policy.rateLimit.burst})
                          </span>
                        ) : (
                          <span className="badge">Disabled</span>
                        )}
                      </td>
                      <td className="mono">v{policy.version}</td>
                      {canWrite ? (
                        <td style={{ textAlign: "right" }}>
                          <div className="row" style={{ justifyContent: "flex-end" }}>
                            <button
                              type="button"
                              className="btn btn-secondary btn-sm"
                              onClick={() => void openEdit(policy)}
                            >
                              <EditIcon size={13} />
                              <span>Edit</span>
                            </button>
                            <button
                              type="button"
                              className="btn btn-danger btn-sm"
                              onClick={() => setDeleteTarget(policy)}
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
              onClick={() => void loadPolicies(nextCursor)}
              disabled={loading}
            >
              Load More Policies
            </button>
          </div>
        ) : null}
      </section>

      {/* Create / Edit Policy Drawer */}
      {drawerOpen ? (
        <div className="backdrop" role="dialog" aria-modal="true" aria-labelledby="policy-drawer-title">
          <div className="drawer">
            <div className="drawer-header">
              <div>
                <h2 id="policy-drawer-title" style={{ fontSize: "1.1rem" }}>
                  {editingPolicy ? `Edit Policy: ${editingPolicy.name}` : "Create WAF Policy"}
                </h2>
                <p className="small">
                  {editingPolicy
                    ? `Optimistic lock ETag: ${editingEtag}`
                    : "Define Coraza CRS thresholds, request body limits, and rate limits."}
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
              {formError ? <ErrorAlert error={formError} /> : null}

              <div className="field">
                <label className="field-label" htmlFor="policy-name">
                  Policy Name
                </label>
                <input
                  id="policy-name"
                  className="input"
                  required
                  maxLength={256}
                  value={form.name}
                  onChange={(e) => setForm({ ...form, name: e.target.value })}
                  placeholder="e.g. Production Strict Blocking"
                />
              </div>

              <div className="field">
                <span className="field-label">Engine Mode</span>
                <div className="segmented" style={{ width: "100%" }}>
                  {(["detectionOnly", "blocking", "off"] as EngineMode[]).map((mode) => (
                    <button
                      key={mode}
                      type="button"
                      className={`segmented-btn ${form.engineMode === mode ? "active" : ""}`}
                      style={{ flex: 1 }}
                      onClick={() => setForm({ ...form, engineMode: mode })}
                    >
                      {engineModeLabel(mode).label}
                    </button>
                  ))}
                </div>
                <span className="field-hint">
                  {form.engineMode === "detectionOnly"
                    ? "Evaluates rules and logs matches without blocking."
                    : form.engineMode === "blocking"
                      ? "Actively blocks requests exceeding anomaly thresholds."
                      : "Disables WAF rule evaluation for attached sites."}
                </span>
              </div>

              <div className="divider" />
              <h3 style={{ fontSize: "0.95rem" }}>OWASP CRS Settings</h3>

              <div className="grid-3">
                <div className="field">
                  <label className="field-label" htmlFor="crs-pl">
                    Paranoia (1–4)
                  </label>
                  <select
                    id="crs-pl"
                    className="select"
                    value={form.crs.paranoiaLevel}
                    onChange={(e) =>
                      setForm({ ...form, crs: { ...form.crs, paranoiaLevel: Number(e.target.value) } })
                    }
                  >
                    <option value={1}>PL1 (Standard)</option>
                    <option value={2}>PL2 (Elevated)</option>
                    <option value={3}>PL3 (High)</option>
                    <option value={4}>PL4 (Maximum)</option>
                  </select>
                </div>

                <div className="field">
                  <label className="field-label" htmlFor="crs-in">
                    Inbound Threshold
                  </label>
                  <input
                    id="crs-in"
                    className="input mono"
                    type="number"
                    min={1}
                    max={100}
                    required
                    value={form.crs.inboundAnomalyThreshold}
                    onChange={(e) =>
                      setForm({
                        ...form,
                        crs: { ...form.crs, inboundAnomalyThreshold: Number(e.target.value) }
                      })
                    }
                  />
                </div>

                <div className="field">
                  <label className="field-label" htmlFor="crs-out">
                    Outbound Threshold
                  </label>
                  <input
                    id="crs-out"
                    className="input mono"
                    type="number"
                    min={1}
                    max={100}
                    required
                    value={form.crs.outboundAnomalyThreshold}
                    onChange={(e) =>
                      setForm({
                        ...form,
                        crs: { ...form.crs, outboundAnomalyThreshold: Number(e.target.value) }
                      })
                    }
                  />
                </div>
              </div>

              <div className="divider" />
              <h3 style={{ fontSize: "0.95rem" }}>Request Body Inspection</h3>

              <label className="checkbox-row">
                <input
                  type="checkbox"
                  checked={form.inspection.requestBodyEnabled}
                  onChange={(e) =>
                    setForm({
                      ...form,
                      inspection: { ...form.inspection, requestBodyEnabled: e.target.checked }
                    })
                  }
                />
                <div>
                  <div style={{ fontWeight: 600 }}>Inspect Request Bodies</div>
                  <div className="small">Enables JSON/form/multipart body inspection up to the limits below.</div>
                </div>
              </label>

              <div className="grid-2">
                <div className="field">
                  <label className="field-label" htmlFor="body-limit">
                    <span>Body Limit (Bytes)</span>
                    <span className="small mono">{formatBytes(form.inspection.requestBodyLimitBytes)}</span>
                  </label>
                  <input
                    id="body-limit"
                    className="input mono"
                    type="number"
                    min={1}
                    max={104857600}
                    required
                    value={form.inspection.requestBodyLimitBytes}
                    onChange={(e) =>
                      setForm({
                        ...form,
                        inspection: { ...form.inspection, requestBodyLimitBytes: Number(e.target.value) }
                      })
                    }
                  />
                </div>

                <div className="field">
                  <label className="field-label" htmlFor="nofiles-limit">
                    <span>No-Files Limit (Bytes)</span>
                    <span className="small mono">{formatBytes(form.inspection.requestBodyNoFilesLimitBytes)}</span>
                  </label>
                  <input
                    id="nofiles-limit"
                    className="input mono"
                    type="number"
                    min={1}
                    max={form.inspection.requestBodyLimitBytes}
                    required
                    value={form.inspection.requestBodyNoFilesLimitBytes}
                    onChange={(e) =>
                      setForm({
                        ...form,
                        inspection: {
                          ...form.inspection,
                          requestBodyNoFilesLimitBytes: Number(e.target.value)
                        }
                      })
                    }
                  />
                </div>
              </div>

              <div className="grid-2">
                <div className="field">
                  <label className="field-label" htmlFor="oversize-action">
                    Oversize Body Action
                  </label>
                  <select
                    id="oversize-action"
                    className="select"
                    value={form.inspection.oversizeAction}
                    onChange={(e) =>
                      setForm({
                        ...form,
                        inspection: { ...form.inspection, oversizeAction: e.target.value as OversizeAction }
                      })
                    }
                  >
                    <option value="reject">reject (Strict)</option>
                    <option value="processPartial">processPartial</option>
                  </select>
                </div>

                <div className="field">
                  <label className="field-label" htmlFor="parser-action">
                    Parser Error Action
                  </label>
                  <select
                    id="parser-action"
                    className="select"
                    value={form.inspection.parserErrorAction}
                    onChange={(e) =>
                      setForm({
                        ...form,
                        inspection: {
                          ...form.inspection,
                          parserErrorAction: e.target.value as ParserErrorAction
                        }
                      })
                    }
                  >
                    <option value="reject">reject (Strict)</option>
                    <option value="logOnly">logOnly</option>
                  </select>
                </div>
              </div>

              <div className="small">
                Note: <code>responseBodyEnabled</code> is locked to <code>false</code> in this milestone.
              </div>

              <div className="divider" />
              <h3 style={{ fontSize: "0.95rem" }}>Rate Limiting</h3>

              <label className="checkbox-row">
                <input
                  type="checkbox"
                  checked={form.rateLimit.enabled}
                  onChange={(e) =>
                    setForm({
                      ...form,
                      rateLimit: { ...form.rateLimit, enabled: e.target.checked }
                    })
                  }
                />
                <div>
                  <div style={{ fontWeight: 600 }}>Enable Per-Client Rate Limit</div>
                  <div className="small">Throttle excessive request bursts on attached websites.</div>
                </div>
              </label>

              <div className="grid-2">
                <div className="field">
                  <label className="field-label" htmlFor="rl-rps">
                    Requests / Sec (1–10,000)
                  </label>
                  <input
                    id="rl-rps"
                    className="input mono"
                    type="number"
                    min={1}
                    max={10000}
                    required
                    value={form.rateLimit.requestsPerSecond}
                    onChange={(e) =>
                      setForm({
                        ...form,
                        rateLimit: { ...form.rateLimit, requestsPerSecond: Number(e.target.value) }
                      })
                    }
                  />
                </div>
                <div className="field">
                  <label className="field-label" htmlFor="rl-burst">
                    Burst Allowance (0–100,000)
                  </label>
                  <input
                    id="rl-burst"
                    className="input mono"
                    type="number"
                    min={0}
                    max={100000}
                    required
                    value={form.rateLimit.burst}
                    onChange={(e) =>
                      setForm({
                        ...form,
                        rateLimit: { ...form.rateLimit, burst: Number(e.target.value) }
                      })
                    }
                  />
                </div>
              </div>

              <div className="drawer-footer" style={{ margin: "0 -1.35rem -1.25rem", marginTop: "0.5rem" }}>
                <button type="button" className="btn btn-secondary" onClick={() => setDrawerOpen(false)}>
                  Cancel
                </button>
                <button type="submit" className="btn btn-primary" disabled={saving}>
                  {saving ? "Saving..." : editingPolicy ? "Save Policy Changes" : "Create Policy"}
                </button>
              </div>
            </form>
          </div>
        </div>
      ) : null}

      {/* Delete Confirmation Modal */}
      {deleteTarget ? (
        <div className="backdrop backdrop-center" role="dialog" aria-modal="true" aria-labelledby="delete-policy-title">
          <div className="modal">
            <div className="modal-header">
              <h2 id="delete-policy-title" style={{ fontSize: "1.05rem" }}>
                Delete WAF Policy
              </h2>
              <button type="button" className="btn btn-ghost btn-icon" onClick={() => setDeleteTarget(null)}>
                <CloseIcon />
              </button>
            </div>
            <div className="modal-body stack-sm">
              <p>
                Are you sure you want to delete policy <strong>{deleteTarget.name}</strong>?
              </p>
              <p className="small">
                Policies currently referenced by any website (including disabled websites) cannot be deleted until
                reassigned.
              </p>
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

export default function PoliciesPage() {
  return (
    <AppShell
      title="WAF Policies"
      subtitle="Configure OWASP CRS paranoia levels, anomaly thresholds, body inspection, and rate limits."
    >
      <PoliciesContent />
    </AppShell>
  );
}
