"use client";

import React, { useEffect, useMemo, useState } from "react";
import { AppShell, useConsole } from "../../components/AppShell";
import { usePreferences } from "../../components/PreferencesContext";
import { EmptyState, ErrorAlert, LoadingRows } from "../../components/StateBanner";
import { ActivityIcon, CloseIcon, RefreshIcon, ShieldIcon } from "../../components/Icons";
import { ApiClientError, apiRequestWithMeta, EventRecord, Website } from "../../lib/api";
import { formatDateTime } from "../../lib/format";

function toQuery(
  filters: {
    siteId: string;
    action: string;
    ruleId: string;
    clientIp: string;
    from: string;
    to: string;
  },
  cursor?: string | null
): string {
  const params = new URLSearchParams();
  if (filters.siteId.trim()) params.set("siteId", filters.siteId.trim());
  if (filters.action.trim()) params.set("action", filters.action.trim());
  if (filters.ruleId.trim()) params.set("ruleId", filters.ruleId.trim());
  if (filters.clientIp.trim()) params.set("clientIp", filters.clientIp.trim());
  if (filters.from.trim()) params.set("from", filters.from.trim());
  if (filters.to.trim()) params.set("to", filters.to.trim());
  params.set("limit", "50");
  if (cursor) params.set("cursor", cursor);
  const text = params.toString();
  return text ? `?${text}` : "";
}

function actionTone(action?: string): "danger" | "warning" | "brand" | "neutral" {
  switch (action) {
    case "blocked":
      return "danger";
    case "rejected":
      return "warning";
    case "allowed":
      return "brand";
    default:
      return "neutral";
  }
}

function EventsContent() {
  const { me, authLoading } = useConsole();
  const { bilingual } = usePreferences();

  const [websites, setWebsites] = useState<Website[]>([]);
  const [filters, setFilters] = useState({
    siteId: "",
    action: "",
    ruleId: "",
    clientIp: "",
    from: "",
    to: ""
  });
  const [loading, setLoading] = useState(false);
  const [collectorUnavailable, setCollectorUnavailable] = useState(false);
  const [hasQueried, setHasQueried] = useState(false);
  const [error, setError] = useState<unknown>(null);
  const [events, setEvents] = useState<EventRecord[]>([]);
  const [nextCursor, setNextCursor] = useState<string | null>(null);
  const [selectedEvent, setSelectedEvent] = useState<EventRecord | null>(null);

  const query = useMemo(() => toQuery(filters), [filters]);

  useEffect(() => {
    if (!authLoading && me) {
      void apiRequestWithMeta<Website[]>("/websites?limit=100")
        .then((res) => setWebsites(res.data))
        .catch(() => setWebsites([]));
    }
  }, [authLoading, me]);

  function applyWindow(minutes: number) {
    const now = new Date();
    const from = new Date(now.getTime() - minutes * 60 * 1000);
    setFilters((prev) => ({
      ...prev,
      from: from.toISOString(),
      to: now.toISOString()
    }));
  }

  async function searchEvents(cursor?: string | null) {
    setLoading(true);
    setError(null);
    setCollectorUnavailable(false);
    setHasQueried(true);
    try {
      const qs = toQuery(filters, cursor);
      const res = await apiRequestWithMeta<EventRecord[] | { items?: EventRecord[] }>(`/events${qs}`);
      const list = Array.isArray(res.data) ? res.data : res.data.items ?? [];
      if (cursor) {
        setEvents((prev) => [...prev, ...list]);
      } else {
        setEvents(list);
      }
      setNextCursor(res.meta?.nextCursor ?? null);
    } catch (err) {
      if (!cursor) setEvents([]);
      setNextCursor(null);
      if (err instanceof ApiClientError && (err.status === 404 || err.code === "NOT_FOUND")) {
        setCollectorUnavailable(true);
      } else {
        setError(err);
      }
    } finally {
      setLoading(false);
    }
  }

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    void searchEvents(null);
  }

  return (
    <div className="stack-md">
      <div className="alert info">
        <ShieldIcon size={18} />
        <div className="small" style={{ color: "var(--ink-secondary)" }}>
          <strong style={{ color: "var(--ink)" }}>Detected vs. Blocked Telemetry:</strong> A CRS rule match in{" "}
          <code>detectionOnly</code> mode can appear on an <code>allowed</code> transaction, whereas{" "}
          <code>blocked</code> indicates confirmed WAF enforcement and <code>rejected</code> indicates transport or
          body-limit rejection. Request bodies and secrets are never returned.
          {bilingual ? " (ডিটেক্টেড বনাম ব্লকড ও রিজেক্টেড ইভেন্ট আলাদাভাবে দেখানো হয়।)" : ""}
        </div>
      </div>

      {/* Filter Bar aligned with docs/api.md §9 */}
      <form onSubmit={handleSubmit} className="card stack-md">
        <div className="row-between">
          <div>
            <h2 className="card-title">Event Query Filters</h2>
            <p className="small mono">GET /api/v1/events{query}</p>
          </div>
          <div className="row">
            <button type="button" className="btn btn-secondary btn-sm" onClick={() => applyWindow(15)}>
              Last 15m
            </button>
            <button type="button" className="btn btn-secondary btn-sm" onClick={() => applyWindow(60)}>
              Last 1h
            </button>
            <button type="button" className="btn btn-secondary btn-sm" onClick={() => applyWindow(1440)}>
              Last 24h
            </button>
            <button
              type="button"
              className="btn btn-ghost btn-sm"
              onClick={() =>
                setFilters({ siteId: "", action: "", ruleId: "", clientIp: "", from: "", to: "" })
              }
            >
              Clear
            </button>
          </div>
        </div>

        <div className="grid-3">
          <div className="field">
            <label className="field-label" htmlFor="ev-site">
              Website (siteId)
            </label>
            {websites.length > 0 ? (
              <select
                id="ev-site"
                className="select"
                value={filters.siteId}
                onChange={(e) => setFilters({ ...filters, siteId: e.target.value })}
              >
                <option value="">All Websites</option>
                {websites.map((w) => (
                  <option key={w.id} value={w.id}>
                    {w.name} ({w.domains[0]})
                  </option>
                ))}
              </select>
            ) : (
              <input
                id="ev-site"
                className="input mono"
                value={filters.siteId}
                onChange={(e) => setFilters({ ...filters, siteId: e.target.value })}
                placeholder="siteId"
              />
            )}
          </div>

          <div className="field">
            <label className="field-label" htmlFor="ev-action">
              Action
            </label>
            <select
              id="ev-action"
              className="select"
              value={filters.action}
              onChange={(e) => setFilters({ ...filters, action: e.target.value })}
            >
              <option value="">All Actions</option>
              <option value="allowed">allowed (incl. DetectionOnly)</option>
              <option value="blocked">blocked (WAF Enforced)</option>
              <option value="rejected">rejected (Limit / Transport)</option>
            </select>
          </div>

          <div className="field">
            <label className="field-label" htmlFor="ev-rule">
              CRS Rule ID (ruleId)
            </label>
            <input
              id="ev-rule"
              className="input mono"
              value={filters.ruleId}
              onChange={(e) => setFilters({ ...filters, ruleId: e.target.value })}
              placeholder="e.g. 942100"
            />
          </div>
        </div>

        <div className="grid-3">
          <div className="field">
            <label className="field-label" htmlFor="ev-ip">
              Client IP (clientIp)
            </label>
            <input
              id="ev-ip"
              className="input mono"
              value={filters.clientIp}
              onChange={(e) => setFilters({ ...filters, clientIp: e.target.value })}
              placeholder="e.g. 192.0.2.10"
            />
          </div>
          <div className="field">
            <label className="field-label" htmlFor="ev-from">
              From (UTC RFC3339, inclusive)
            </label>
            <input
              id="ev-from"
              className="input mono"
              value={filters.from}
              onChange={(e) => setFilters({ ...filters, from: e.target.value })}
              placeholder="2026-11-11T00:00:00Z"
            />
          </div>
          <div className="field">
            <label className="field-label" htmlFor="ev-to">
              To (UTC RFC3339, exclusive)
            </label>
            <input
              id="ev-to"
              className="input mono"
              value={filters.to}
              onChange={(e) => setFilters({ ...filters, to: e.target.value })}
              placeholder="2026-11-12T00:00:00Z"
            />
          </div>
        </div>

        <div className="row">
          <button type="submit" className="btn btn-primary" disabled={loading}>
            <RefreshIcon size={15} />
            <span>{loading ? "Querying Telemetry..." : "Run Event Query"}</span>
          </button>
        </div>
      </form>

      {error ? <ErrorAlert error={error} onRetry={() => void searchEvents(null)} /> : null}

      {collectorUnavailable ? (
        <section className="card">
          <div className="alert warning" style={{ border: 0, background: "transparent", padding: 0 }}>
            <ActivityIcon size={20} />
            <div className="stack-sm">
              <div className="row">
                <strong style={{ fontSize: "1rem" }}>Event Collector Unavailable (HTTP 404)</strong>
                <span className="badge warning">Backend Route Pending</span>
              </div>
              <p className="small">
                The running Go management API returned <code>404 NOT_FOUND</code> for <code>/api/v1/events</code>.
                Per Ariba Shield telemetry integrity rules, an unavailable collector is explicitly reported here rather
                than displaying zero events.
              </p>
            </div>
          </div>
        </section>
      ) : null}

      <section className="card card-flush">
        <div className="card-header-flush">
          <h2 className="card-title">Security Transactions ({events.length})</h2>
          <span className="badge">{hasQueried ? "Queried" : "Awaiting Query"}</span>
        </div>

        {loading && events.length === 0 ? (
          <LoadingRows count={4} />
        ) : events.length === 0 ? (
          <EmptyState
            icon={<ActivityIcon size={20} />}
            title={
              collectorUnavailable
                ? "Collector not connected"
                : hasQueried
                  ? "No matching security events"
                  : "Ready to query security events"
            }
            description={
              collectorUnavailable
                ? "Once the WAF audit-log ingestion pipeline is enabled on /api/v1/events, redacted transactions will appear here."
                : hasQueried
                  ? "No redacted event records matched the selected time window, siteId, action, clientIp, or ruleId filter."
                  : "Select optional filters or a time window preset above and click Run Event Query."
            }
          />
        ) : (
          <div className="table-wrap">
            <table className="data-table">
              <thead>
                <tr>
                  <th>Timestamp</th>
                  <th>Action</th>
                  <th>Method / Path</th>
                  <th>Rules Matched</th>
                  <th>Client IP</th>
                  <th style={{ textAlign: "right" }}>Detail</th>
                </tr>
              </thead>
              <tbody>
                {events.map((ev) => {
                  const firstMatch = ev.matches?.[0];
                  const ruleBadge = firstMatch?.ruleId ?? ev.ruleId;
                  return (
                    <tr key={ev.id}>
                      <td className="mono small">{formatDateTime(ev.timestamp)}</td>
                      <td>
                        <span className={`badge ${actionTone(ev.action)}`}>{ev.action ?? "unknown"}</span>
                        {ev.statusCode ? (
                          <span className="small mono" style={{ marginLeft: 6 }}>
                            {ev.statusCode}
                          </span>
                        ) : null}
                      </td>
                      <td className="mono small">
                        <strong>{ev.method ?? "—"}</strong> {ev.path ?? "—"}
                      </td>
                      <td className="small">
                        {ruleBadge ? <span className="badge mono">{ruleBadge}</span> : "—"}
                        {ev.matches && ev.matches.length > 1 ? (
                          <span className="small" style={{ marginLeft: 6 }}>
                            +{ev.matches.length - 1} more
                          </span>
                        ) : null}
                      </td>
                      <td className="mono small">{ev.clientIp ?? "—"}</td>
                      <td style={{ textAlign: "right" }}>
                        <button
                          type="button"
                          className="btn btn-secondary btn-sm"
                          onClick={() => setSelectedEvent(ev)}
                        >
                          Inspect
                        </button>
                      </td>
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
              onClick={() => void searchEvents(nextCursor)}
              disabled={loading}
            >
              Load More Events
            </button>
          </div>
        ) : null}
      </section>

      {selectedEvent ? (
        <div className="backdrop" role="dialog" aria-modal="true" aria-labelledby="event-drawer-title">
          <div className="drawer">
            <div className="drawer-header">
              <div>
                <h2 id="event-drawer-title" style={{ fontSize: "1.1rem" }}>
                  Redacted Transaction Detail
                </h2>
                <p className="small mono">{selectedEvent.transactionId ?? selectedEvent.id}</p>
              </div>
              <button
                type="button"
                className="btn btn-ghost btn-icon"
                onClick={() => setSelectedEvent(null)}
                aria-label="Close drawer"
              >
                <CloseIcon />
              </button>
            </div>
            <div className="drawer-body stack-md">
              <div className="card stack-sm" style={{ background: "var(--surface-2)" }}>
                <div className="row-between small">
                  <span>Timestamp</span>
                  <span className="mono">{formatDateTime(selectedEvent.timestamp)}</span>
                </div>
                <div className="row-between small">
                  <span>Action / HTTP Status</span>
                  <span className="row">
                    <span className={`badge ${actionTone(selectedEvent.action)}`}>
                      {selectedEvent.action ?? "unknown"}
                    </span>
                    {selectedEvent.statusCode ? <span className="mono">{selectedEvent.statusCode}</span> : null}
                  </span>
                </div>
                <div className="row-between small">
                  <span>Engine Mode</span>
                  <span className="mono">{selectedEvent.engineMode ?? "—"}</span>
                </div>
                <div className="row-between small">
                  <span>Request</span>
                  <span className="mono">
                    {selectedEvent.method ?? "—"} {selectedEvent.path ?? "—"}
                  </span>
                </div>
                <div className="row-between small">
                  <span>Site ID</span>
                  <span className="mono">{selectedEvent.siteName ?? selectedEvent.siteId ?? "—"}</span>
                </div>
                <div className="row-between small">
                  <span>Client IP</span>
                  <span className="mono">{selectedEvent.clientIp ?? "—"}</span>
                </div>
                {selectedEvent.appliedRevisionId ? (
                  <div className="row-between small">
                    <span>Applied Revision</span>
                    <span className="mono">{selectedEvent.appliedRevisionId}</span>
                  </div>
                ) : null}
              </div>

              <div className="field">
                <span className="field-label">Matched Rules</span>
                {selectedEvent.matches && selectedEvent.matches.length > 0 ? (
                  <div className="stack-sm">
                    {selectedEvent.matches.map((m, idx) => (
                      <div key={idx} className="card stack-sm" style={{ background: "var(--code-bg)", padding: "0.75rem" }}>
                        <div className="row-between">
                          <span className="badge mono">Rule {m.ruleId}</span>
                          {m.severity ? <span className="badge warning">{m.severity}</span> : null}
                        </div>
                        <div className="small">{m.message ?? "(redacted)"}</div>
                      </div>
                    ))}
                  </div>
                ) : (
                  <div className="card mono small" style={{ background: "var(--code-bg)" }}>
                    {selectedEvent.message ?? "(no rule matches or redacted)"}
                  </div>
                )}
              </div>
            </div>
          </div>
        </div>
      ) : null}
    </div>
  );
}

export default function EventsPage() {
  return (
    <AppShell
      title="Security Events"
      subtitle="Investigate redacted WAF detections, confirmed blocks, and CRS rule matches."
    >
      <EventsContent />
    </AppShell>
  );
}