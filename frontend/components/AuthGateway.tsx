"use client";

import React, { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { apiRequest, getCsrfToken, MeResponse, SetupStatus } from "../lib/api";
import {
  CheckCircleIcon,
  EyeIcon,
  EyeOffIcon,
  LockIcon,
  MoonIcon,
  ShieldIcon,
  SunIcon,
  UsersIcon
} from "./Icons";
import { usePreferences } from "./PreferencesContext";
import { ErrorAlert, SuccessAlert } from "./StateBanner";

export function AuthGateway({ initialTab = "auto" }: { initialTab?: "auto" | "login" | "enroll" }) {
  const router = useRouter();
  const { theme, toggleTheme } = usePreferences();

  const [tab, setTab] = useState<"login" | "enroll">(initialTab === "enroll" ? "enroll" : "login");
  const [submitting, setSubmitting] = useState(false);

  const [setupStatus, setSetupStatus] = useState<SetupStatus | null>(null);
  const [existingMe, setExistingMe] = useState<MeResponse | null>(null);

  const [error, setError] = useState<unknown>(null);
  const [success, setSuccess] = useState("");

  const [showLoginPass, setShowLoginPass] = useState(false);
  const [showEnrollPass, setShowEnrollPass] = useState(false);

  const [loginForm, setLoginForm] = useState({ username: "", password: "" });
  const [enrollForm, setEnrollForm] = useState({
    enrollmentToken: "",
    username: "admin",
    displayName: "WAF Administrator",
    password: ""
  });

  const probeSetup = useCallback(async () => {
    try {
      const statusRes = await apiRequest<SetupStatus>("/setup/status");
      setSetupStatus(statusRes);
      if (initialTab === "auto" && statusRes.enrollmentRequired) {
        setTab("enroll");
      }
      try {
        const me = await apiRequest<MeResponse>("/auth/me");
        setExistingMe(me);
      } catch {
        setExistingMe(null);
      }
    } catch {
      // Keep login card usable even if initial status check fails.
    }
  }, [initialTab]);

  useEffect(() => {
    void probeSetup();
  }, [probeSetup]);

  async function handleLogin(e: React.FormEvent) {
    e.preventDefault();
    setSubmitting(true);
    setError(null);
    setSuccess("");
    try {
      const csrfToken = await getCsrfToken();
      await apiRequest("/auth/login", {
        method: "POST",
        csrfToken,
        body: {
          username: loginForm.username.trim(),
          password: loginForm.password
        }
      });
      await getCsrfToken();
      router.push("/dashboard");
    } catch (err) {
      setError(err);
    } finally {
      setSubmitting(false);
    }
  }

  async function handleEnroll(e: React.FormEvent) {
    e.preventDefault();
    setSubmitting(true);
    setError(null);
    setSuccess("");
    try {
      const csrfToken = await getCsrfToken();
      await apiRequest("/setup/enroll", {
        method: "POST",
        csrfToken,
        idempotencyKey: crypto.randomUUID(),
        body: {
          enrollmentToken: enrollForm.enrollmentToken.trim(),
          username: enrollForm.username.trim(),
          displayName: enrollForm.displayName.trim(),
          password: enrollForm.password
        }
      });
      setSuccess("Administrator enrolled. Sign in to continue.");
      setLoginForm({ username: enrollForm.username.trim(), password: "" });
      setEnrollForm((prev) => ({ ...prev, enrollmentToken: "", password: "" }));
      setTab("login");
      await probeSetup();
    } catch (err) {
      setError(err);
    } finally {
      setSubmitting(false);
    }
  }

  const passLen = enrollForm.password.length;
  const passValid = passLen >= 12 && passLen <= 128;

  return (
    <div className="auth-stage">
      <div className="auth-corner-actions">
        <button
          type="button"
          onClick={toggleTheme}
          className="btn btn-secondary btn-icon"
          aria-label="Toggle theme"
          title="Toggle theme"
        >
          {theme === "dark" ? <SunIcon size={16} /> : <MoonIcon size={16} />}
        </button>
      </div>

      {/* Subtle layered geometric horizon wave */}
      <svg
        className="auth-wave"
        viewBox="0 0 1440 180"
        preserveAspectRatio="none"
        aria-hidden="true"
      >
        <path
          d="M0,115 C320,40 620,155 960,95 C1180,55 1340,85 1440,105 L1440,180 L0,180 Z"
          fill="rgba(14, 165, 233, 0.14)"
        />
        <path
          d="M0,140 C360,75 740,175 1120,110 C1280,82 1380,100 1440,115 L1440,180 L0,180 Z"
          fill="rgba(37, 99, 235, 0.18)"
        />
      </svg>

      <main className="auth-card">
        <div className="auth-header">
          <div className="auth-shield-ring">
            <ShieldIcon size={28} />
          </div>
          <h1 className="auth-brand">
            ARIBA <span className="auth-brand-accent">SHIELD</span>
          </h1>
          <p className="auth-tagline">
            {tab === "login"
              ? "WAF Control Plane & Telemetry Cockpit"
              : "First-Time Administrator Enrollment"}
          </p>
        </div>

        {existingMe ? (
          <div className="alert success" style={{ marginBottom: "1rem", alignItems: "center" }}>
            <CheckCircleIcon size={17} />
            <div style={{ flex: 1, minWidth: 0 }}>
              <div style={{ fontWeight: 650, fontSize: "0.85rem" }}>
                Signed in as {existingMe.user.displayName}
              </div>
            </div>
            <Link href="/dashboard" className="btn btn-primary btn-sm">
              Open Console
            </Link>
          </div>
        ) : null}

        {success ? (
          <div style={{ marginBottom: "1rem" }}>
            <SuccessAlert message={success} onDismiss={() => setSuccess("")} />
          </div>
        ) : null}
        {error ? (
          <div style={{ marginBottom: "1rem" }}>
            <ErrorAlert error={error} />
          </div>
        ) : null}

        {tab === "login" ? (
          <form onSubmit={handleLogin} className="form-stack">
            <div className="field">
              <label className="auth-label" htmlFor="login-username">
                <span className="auth-label-icon">
                  <UsersIcon size={14} />
                </span>
                <span>Username</span>
              </label>
              <input
                id="login-username"
                className="auth-input"
                autoComplete="username"
                required
                value={loginForm.username}
                onChange={(e) => setLoginForm({ ...loginForm, username: e.target.value })}
                placeholder="Enter operator username"
              />
            </div>

            <div className="field">
              <label className="auth-label" htmlFor="login-password">
                <span className="auth-label-icon">
                  <LockIcon size={14} />
                </span>
                <span>Password</span>
              </label>
              <div className="input-wrap">
                <input
                  id="login-password"
                  className="auth-input"
                  type={showLoginPass ? "text" : "password"}
                  autoComplete="current-password"
                  required
                  value={loginForm.password}
                  onChange={(e) => setLoginForm({ ...loginForm, password: e.target.value })}
                  placeholder="Enter security passphrase"
                />
                <button
                  type="button"
                  className="input-action"
                  onClick={() => setShowLoginPass((v) => !v)}
                  aria-label={showLoginPass ? "Hide password" : "Show password"}
                >
                  {showLoginPass ? <EyeOffIcon /> : <EyeIcon />}
                </button>
              </div>
            </div>

            <button
              type="submit"
              className="auth-btn"
              disabled={submitting || !loginForm.username.trim() || !loginForm.password}
            >
              <span>{submitting ? "Authenticating..." : "Access Control Plane"}</span>
              <span aria-hidden="true">→</span>
            </button>
          </form>
        ) : (
          <form onSubmit={handleEnroll} className="form-stack">
            {setupStatus?.enrollmentRequired === false ? (
              <div className="alert info">
                <div className="small">
                  Initial administrator setup is already complete.{" "}
                  <button
                    type="button"
                    onClick={() => setTab("login")}
                    style={{
                      background: "transparent",
                      border: 0,
                      color: "var(--brand)",
                      fontWeight: 700,
                      cursor: "pointer",
                      padding: 0,
                      textDecoration: "underline"
                    }}
                  >
                    Return to Sign In
                  </button>
                </div>
              </div>
            ) : null}

            <div className="field">
              <label className="auth-label" htmlFor="enroll-token">
                <span>One-Time Enrollment Token</span>
              </label>
              <input
                id="enroll-token"
                className="auth-input mono"
                required
                value={enrollForm.enrollmentToken}
                onChange={(e) => setEnrollForm({ ...enrollForm, enrollmentToken: e.target.value })}
                placeholder="Paste 43-char CLI token"
              />
            </div>

            <div className="grid-2">
              <div className="field">
                <label className="auth-label" htmlFor="enroll-username">
                  Username
                </label>
                <input
                  id="enroll-username"
                  className="auth-input"
                  required
                  value={enrollForm.username}
                  onChange={(e) => setEnrollForm({ ...enrollForm, username: e.target.value })}
                />
              </div>
              <div className="field">
                <label className="auth-label" htmlFor="enroll-display">
                  Display Name
                </label>
                <input
                  id="enroll-display"
                  className="auth-input"
                  required
                  value={enrollForm.displayName}
                  onChange={(e) => setEnrollForm({ ...enrollForm, displayName: e.target.value })}
                />
              </div>
            </div>

            <div className="field">
              <label className="auth-label" htmlFor="enroll-password">
                <span>Password</span>
                <span className="small mono" style={{ marginLeft: "auto" }}>
                  {passLen}/12–128
                </span>
              </label>
              <div className="input-wrap">
                <input
                  id="enroll-password"
                  className="auth-input"
                  type={showEnrollPass ? "text" : "password"}
                  required
                  minLength={12}
                  maxLength={128}
                  value={enrollForm.password}
                  onChange={(e) => setEnrollForm({ ...enrollForm, password: e.target.value })}
                  placeholder="Minimum 12 characters"
                />
                <button
                  type="button"
                  className="input-action"
                  onClick={() => setShowEnrollPass((v) => !v)}
                  aria-label={showEnrollPass ? "Hide password" : "Show password"}
                >
                  {showEnrollPass ? <EyeOffIcon /> : <EyeIcon />}
                </button>
              </div>
            </div>

            <button
              type="submit"
              className="auth-btn"
              disabled={submitting || setupStatus?.enrollmentRequired === false || !passValid}
            >
              <span>{submitting ? "Enrolling..." : "Enroll Administrator"}</span>
              <span aria-hidden="true">→</span>
            </button>

            <div style={{ textAlign: "center" }}>
              <button
                type="button"
                onClick={() => {
                  setTab("login");
                  setError(null);
                }}
                className="btn btn-ghost btn-sm"
              >
                Back to Sign In
              </button>
            </div>
          </form>
        )}

        <div className="auth-footer">
          <LockIcon size={12} />
          <span>End-to-End Protected Management Session</span>
          {setupStatus?.enrollmentRequired && tab === "login" ? (
            <>
              <span>•</span>
              <button
                type="button"
                onClick={() => {
                  setTab("enroll");
                  setError(null);
                }}
                style={{
                  background: "transparent",
                  border: 0,
                  color: "#38bdf8",
                  cursor: "pointer",
                  fontSize: "inherit",
                  fontWeight: 600,
                  padding: 0
                }}
              >
                First-time setup
              </button>
            </>
          ) : null}
        </div>
      </main>
    </div>
  );
}
