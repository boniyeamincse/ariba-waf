"use client";

import React, { useCallback, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { AppShell, useConsole } from "../../components/AppShell";
import { EmptyState, ErrorAlert, LoadingRows, SuccessAlert } from "../../components/StateBanner";
import {
  EyeIcon,
  EyeOffIcon,
  KeyIcon,
  LockIcon,
  RefreshIcon,
  ShieldIcon,
  TrashIcon
} from "../../components/Icons";
import { apiMutate, apiRequest, getCsrfToken, SessionItem } from "../../lib/api";
import { formatDateTime, formatTimeRemaining, roleBadgeMeta } from "../../lib/format";

function AccountContent() {
  const router = useRouter();
  const { me, authLoading, refreshShell } = useConsole();

  const [loading, setLoading] = useState(true);
  const [sessions, setSessions] = useState<SessionItem[]>([]);
  const [error, setError] = useState<unknown>(null);
  const [success, setSuccess] = useState("");

  const [reauthPassword, setReauthPassword] = useState("");
  const [reauthing, setReauthing] = useState(false);

  const [currentPassword, setCurrentPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [changingPass, setChangingPass] = useState(false);
  const [showNewPass, setShowNewPass] = useState(false);

  const [revokingId, setRevokingId] = useState<string | null>(null);

  const loadSessions = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const data = await apiRequest<SessionItem[]>("/auth/sessions");
      setSessions(data);
    } catch (err) {
      setError(err);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    if (!authLoading && me) {
      void loadSessions();
    } else if (!authLoading && !me) {
      setLoading(false);
    }
  }, [authLoading, me, loadSessions]);

  async function handleReauth(e: React.FormEvent) {
    e.preventDefault();
    setReauthing(true);
    setError(null);
    setSuccess("");
    try {
      await apiMutate<{ recentAuthSeconds: number }>("/auth/reauthenticate", {
        method: "POST",
        body: { password: reauthPassword }
      });
      await getCsrfToken();
      setReauthPassword("");
      setSuccess("Re-authentication verified. Session rotated and recent-auth window granted (300s).");
      await Promise.all([loadSessions(), refreshShell()]);
    } catch (err) {
      setError(err);
    } finally {
      setReauthing(false);
    }
  }

  async function handleChangePassword(e: React.FormEvent) {
    e.preventDefault();
    setChangingPass(true);
    setError(null);
    setSuccess("");
    try {
      await apiMutate("/auth/password", {
        method: "POST",
        body: {
          currentPassword,
          newPassword
        }
      });
      setCurrentPassword("");
      setNewPassword("");
      setSessions([]);
      setSuccess("Password updated and all active sessions revoked. Redirecting to Sign In...");
      setTimeout(() => {
        router.push("/");
      }, 1200);
    } catch (err) {
      setError(err);
    } finally {
      setChangingPass(false);
    }
  }

  async function handleRevokeSession(session: SessionItem) {
    setRevokingId(session.id);
    setError(null);
    setSuccess("");
    try {
      await apiMutate(`/auth/sessions/${session.id}`, { method: "DELETE" });
      if (session.current) {
        router.push("/");
        return;
      }
      setSuccess(`Session ${session.id.slice(0, 12)}… revoked.`);
      await loadSessions();
    } catch (err) {
      setError(err);
    } finally {
      setRevokingId(null);
    }
  }

  const roleMeta = me ? roleBadgeMeta(me.user.role) : null;
  const newPassValid = newPassword.length >= 12 && newPassword.length <= 128;

  return (
    <div className="stack-md">
      {success ? <SuccessAlert message={success} onDismiss={() => setSuccess("")} /> : null}
      {error ? <ErrorAlert error={error} onRetry={() => void loadSessions()} /> : null}

      {me ? (
        <section className="card row-between">
          <div className="row" style={{ gap: "0.9rem" }}>
            <div className="brand-mark">
              <ShieldIcon size={18} />
            </div>
            <div>
              <div className="row" style={{ gap: 8 }}>
                <strong style={{ fontSize: "1.05rem" }}>{me.user.displayName}</strong>
                {roleMeta ? <span className={`badge ${roleMeta.tone}`}>{roleMeta.label}</span> : null}
              </div>
              <div className="small mono">
                username: {me.user.username} · account version: v{me.user.version}
              </div>
            </div>
          </div>
          <div style={{ textAlign: "right" }}>
            <div className="small">Current Session Expires</div>
            <div className="mono" style={{ fontWeight: 650 }}>
              {formatDateTime(me.expiresAt)} ({formatTimeRemaining(me.expiresAt)})
            </div>
          </div>
        </section>
      ) : null}

      <section className="grid-2">
        <form onSubmit={handleReauth} className="card form-stack">
          <div className="row-between">
            <div>
              <h2 className="card-title">Step-Up Re-Authentication</h2>
              <p className="small">Rotate session and refresh the 5-minute recent-auth window.</p>
            </div>
            <LockIcon size={18} />
          </div>

          <div className="field">
            <label className="field-label" htmlFor="reauth-pass">
              Current Password
            </label>
            <input
              id="reauth-pass"
              className="input"
              type="password"
              required
              autoComplete="current-password"
              value={reauthPassword}
              onChange={(e) => setReauthPassword(e.target.value)}
              placeholder="Enter current password"
            />
          </div>

          <button type="submit" className="btn btn-primary" disabled={reauthing || !reauthPassword}>
            <LockIcon size={15} />
            <span>{reauthing ? "Verifying..." : "Verify & Rotate Session"}</span>
          </button>
        </form>

        <form onSubmit={handleChangePassword} className="card form-stack">
          <div className="row-between">
            <div>
              <h2 className="card-title">Change Password</h2>
              <p className="small">Updating password revokes all active sessions for your account.</p>
            </div>
            <KeyIcon size={18} />
          </div>

          <div className="field">
            <label className="field-label" htmlFor="curr-pass">
              Current Password
            </label>
            <input
              id="curr-pass"
              className="input"
              type="password"
              required
              autoComplete="current-password"
              value={currentPassword}
              onChange={(e) => setCurrentPassword(e.target.value)}
            />
          </div>

          <div className="field">
            <label className="field-label" htmlFor="new-pass">
              <span>New Password</span>
              <span className={`badge ${newPassValid ? "brand" : "neutral"}`}>
                {newPassword.length} / 12–128 chars
              </span>
            </label>
            <div className="input-wrap">
              <input
                id="new-pass"
                className="input"
                type={showNewPass ? "text" : "password"}
                required
                minLength={12}
                maxLength={128}
                autoComplete="new-password"
                value={newPassword}
                onChange={(e) => setNewPassword(e.target.value)}
                placeholder="Minimum 12 characters"
              />
              <button
                type="button"
                className="input-action"
                onClick={() => setShowNewPass((v) => !v)}
                aria-label={showNewPass ? "Hide password" : "Show password"}
              >
                {showNewPass ? <EyeOffIcon /> : <EyeIcon />}
              </button>
            </div>
          </div>

          <button
            type="submit"
            className="btn btn-primary"
            disabled={changingPass || !currentPassword || !newPassValid}
          >
            <KeyIcon size={15} />
            <span>{changingPass ? "Updating Password..." : "Update Password & Revoke Sessions"}</span>
          </button>
        </form>
      </section>

      {/* Active Sessions Table */}
      <section className="card card-flush">
        <div className="card-header-flush">
          <div>
            <h2 className="card-title">Active Management Sessions ({sessions.length} / 10 max)</h2>
            <p className="small">Idle timeout is 30 minutes; absolute lifetime is 12 hours.</p>
          </div>
          <button type="button" onClick={() => void loadSessions()} className="btn btn-secondary btn-sm" disabled={loading}>
            <RefreshIcon size={14} />
            <span>Refresh</span>
          </button>
        </div>

        {loading && sessions.length === 0 ? (
          <LoadingRows count={3} />
        ) : sessions.length === 0 ? (
          <EmptyState
            icon={<LockIcon size={20} />}
            title="No active sessions"
            description="No live sessions were returned for this account."
          />
        ) : (
          <div className="table-wrap">
            <table className="data-table">
              <thead>
                <tr>
                  <th>Session ID</th>
                  <th>Created</th>
                  <th>Last Seen</th>
                  <th>Expires</th>
                  <th style={{ textAlign: "right" }}>Action</th>
                </tr>
              </thead>
              <tbody>
                {sessions.map((session) => (
                  <tr key={session.id}>
                    <td>
                      <div className="row" style={{ gap: 6 }}>
                        <span className="mono">{session.id.slice(0, 16)}…</span>
                        {session.current ? <span className="badge brand">Current Browser</span> : null}
                      </div>
                    </td>
                    <td className="mono small">{formatDateTime(session.createdAt)}</td>
                    <td className="mono small">{formatDateTime(session.lastSeenAt)}</td>
                    <td className="mono small">{formatDateTime(session.expiresAt)}</td>
                    <td style={{ textAlign: "right" }}>
                      <button
                        type="button"
                        className="btn btn-danger btn-sm"
                        disabled={revokingId === session.id}
                        onClick={() => void handleRevokeSession(session)}
                      >
                        <TrashIcon size={13} />
                        <span>
                          {revokingId === session.id
                            ? "Revoking..."
                            : session.current
                              ? "Revoke & Sign Out"
                              : "Revoke"}
                        </span>
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>
    </div>
  );
}

export default function AccountPage() {
  return (
    <AppShell
      title="Account Security"
      subtitle="Manage active browser sessions, step-up re-authentication, and credential rotation."
    >
      <AccountContent />
    </AppShell>
  );
}