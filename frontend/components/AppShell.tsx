"use client";

import React, { createContext, useCallback, useContext, useEffect, useState } from "react";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import {
  apiMutate,
  apiRequest,
  ApiClientError,
  ConfigurationStatus,
  HealthProbeResult,
  MeResponse,
  probeHealth
} from "../lib/api";
import { formatTimeRemaining, roleBadgeMeta } from "../lib/format";
import {
  ActivityIcon,
  DashboardIcon,
  GlobeIcon,
  LockIcon,
  LogOutIcon,
  MenuIcon,
  MoonIcon,
  RefreshIcon,
  SettingsIcon,
  ShieldIcon,
  SlidersIcon,
  SunIcon,
  UsersIcon,
  CloseIcon
} from "./Icons";
import { usePreferences } from "./PreferencesContext";

type ConsoleContextValue = {
  me: MeResponse | null;
  configStatus: ConfigurationStatus | null;
  health: HealthProbeResult | null;
  authLoading: boolean;
  authError: unknown;
  refreshShell: () => Promise<void>;
};

const ConsoleContext = createContext<ConsoleContextValue>({
  me: null,
  configStatus: null,
  health: null,
  authLoading: true,
  authError: null,
  refreshShell: async () => {}
});

export function useConsole() {
  return useContext(ConsoleContext);
}

type NavItem = {
  href: string;
  label: string;
  bn: string;
  icon: React.ReactNode;
  adminOnly?: boolean;
};

const NAV_GROUPS: { title: string; items: NavItem[] }[] = [
  {
    title: "Operations",
    items: [
      { href: "/dashboard", label: "Overview", bn: "ওভারভিউ", icon: <DashboardIcon size={17} /> },
      { href: "/events", label: "Security Events", bn: "ইভেন্টস", icon: <ActivityIcon size={17} /> }
    ]
  },
  {
    title: "Traffic & Protection",
    items: [
      { href: "/websites", label: "Websites", bn: "ওয়েবসাইট", icon: <GlobeIcon size={17} /> },
      { href: "/policies", label: "WAF Policies", bn: "পলিসি", icon: <SlidersIcon size={17} /> }
    ]
  },
  {
    title: "Platform & Access",
    items: [
      { href: "/settings", label: "Config & Proxy", bn: "কনফিগ ও প্রক্সি", icon: <SettingsIcon size={17} /> },
      { href: "/users", label: "Users & RBAC", bn: "ইউজার", icon: <UsersIcon size={17} />, adminOnly: true },
      { href: "/account", label: "Account Security", bn: "অ্যাকাউন্ট", icon: <LockIcon size={17} /> }
    ]
  }
];

export function AppShell({
  title,
  subtitle,
  actions,
  children
}: {
  title: string;
  subtitle?: string;
  actions?: React.ReactNode;
  children: React.ReactNode;
}) {
  const pathname = usePathname();
  const router = useRouter();
  const { theme, toggleTheme, bilingual, toggleBilingual } = usePreferences();

  const [sidebarOpen, setSidebarOpen] = useState(false);
  const [me, setMe] = useState<MeResponse | null>(null);
  const [configStatus, setConfigStatus] = useState<ConfigurationStatus | null>(null);
  const [health, setHealth] = useState<HealthProbeResult | null>(null);
  const [authLoading, setAuthLoading] = useState(true);
  const [authError, setAuthError] = useState<unknown>(null);
  const [loggingOut, setLoggingOut] = useState(false);

  const refreshShell = useCallback(async () => {
    try {
      const [healthRes, meRes] = await Promise.all([
        probeHealth(),
        apiRequest<MeResponse>("/auth/me")
      ]);
      setHealth(healthRes);
      setMe(meRes);
      setAuthError(null);
      try {
        const cfg = await apiRequest<ConfigurationStatus>("/configuration/status");
        setConfigStatus(cfg);
      } catch {
        setConfigStatus(null);
      }
    } catch (err) {
      setMe(null);
      setConfigStatus(null);
      setAuthError(err);
      const healthRes = await probeHealth();
      setHealth(healthRes);
    } finally {
      setAuthLoading(false);
    }
  }, []);

  useEffect(() => {
    void refreshShell();
  }, [refreshShell]);

  async function handleLogout() {
    setLoggingOut(true);
    try {
      await apiMutate("/auth/logout", { method: "POST" });
    } catch {
      // Clear client state and route to gateway regardless
    } finally {
      setMe(null);
      setConfigStatus(null);
      setLoggingOut(false);
      router.push("/");
    }
  }

  const isUnauthenticated =
    !authLoading &&
    authError instanceof ApiClientError &&
    (authError.status === 401 || authError.code === "AUTH_REQUIRED" || authError.code === "SESSION_EXPIRED");

  const roleMeta = me ? roleBadgeMeta(me.user.role) : null;

  return (
    <ConsoleContext.Provider value={{ me, configStatus, health, authLoading, authError, refreshShell }}>
      <div className="app-shell">
        <div
          className={`sidebar-overlay ${sidebarOpen ? "open" : ""}`}
          onClick={() => setSidebarOpen(false)}
          aria-hidden="true"
        />

        <aside className={`sidebar ${sidebarOpen ? "open" : ""}`} aria-label="Primary Navigation">
          <div className="sidebar-brand">
            <div className="brand-mark">
              <ShieldIcon size={20} />
            </div>
            <div style={{ flex: 1, minWidth: 0 }}>
              <div className="brand-title">Ariba Shield</div>
              <div className="brand-sub">WAF Control Plane</div>
            </div>
            <button
              type="button"
              className="btn btn-ghost btn-icon mobile-only"
              onClick={() => setSidebarOpen(false)}
              aria-label="Close navigation"
            >
              <CloseIcon size={18} />
            </button>
          </div>

          <nav className="sidebar-nav">
            {NAV_GROUPS.map((group) => {
              const visibleItems = group.items.filter(
                (item) => !item.adminOnly || !me || me.user.role === "admin"
              );
              return (
                <div key={group.title}>
                  <div className="nav-group-label">{group.title}</div>
                  <div className="nav-list">
                    {visibleItems.map((item) => {
                      const active = pathname === item.href || pathname?.startsWith(item.href + "/");
                      return (
                        <Link
                          key={item.href}
                          href={item.href}
                          className={`nav-item ${active ? "active" : ""}`}
                          onClick={() => setSidebarOpen(false)}
                        >
                          {item.icon}
                          <span style={{ flex: 1 }}>{item.label}</span>
                          {bilingual ? (
                            <span style={{ fontSize: "0.7rem", opacity: 0.72 }}>{item.bn}</span>
                          ) : null}
                        </Link>
                      );
                    })}
                  </div>
                </div>
              );
            })}
          </nav>

          <div className="sidebar-footer">
            <div className="health-widget">
              <div className="health-row">
                <span style={{ fontWeight: 600, color: "var(--ink)" }}>Control Plane</span>
                <button
                  type="button"
                  onClick={() => void refreshShell()}
                  className="btn btn-ghost btn-icon"
                  style={{ padding: 2 }}
                  title="Refresh status"
                  aria-label="Refresh control plane status"
                >
                  <RefreshIcon size={13} />
                </button>
              </div>
              <div className="health-row">
                <span>API Liveness</span>
                <span className="row" style={{ gap: 6 }}>
                  <span className={`dot ${health?.live.ok ? "ok" : "err"}`} />
                  <span className="mono">{health?.live.ok ? "200 OK" : "Offline"}</span>
                </span>
              </div>
              <div className="health-row">
                <span>WAF Readiness</span>
                <span className="row" style={{ gap: 6 }}>
                  <span className={`dot ${health?.ready.ok ? "ok" : "warn"}`} />
                  <span className="mono">{health?.ready.ok ? "Ready" : "503 Split"}</span>
                </span>
              </div>
              {configStatus ? (
                <div className="health-row" style={{ paddingTop: 4, borderTop: "1px solid var(--border)" }}>
                  <span>Desired Gen</span>
                  <span className="mono" style={{ fontWeight: 600 }}>
                    #{configStatus.desiredGeneration}
                    {configStatus.pendingChanges ? " (Pending)" : ""}
                  </span>
                </div>
              ) : null}
            </div>
          </div>
        </aside>

        <div className="main-column">
          <header className="topbar">
            <div className="topbar-left">
              <button
                type="button"
                className="btn btn-ghost btn-icon mobile-only"
                onClick={() => setSidebarOpen(true)}
                aria-label="Open navigation"
              >
                <MenuIcon size={20} />
              </button>
              <div className="row" style={{ gap: "0.55rem" }}>
                <span className="small" style={{ fontWeight: 600 }}>
                  Ariba Shield
                </span>
                <span className="small">/</span>
                <span style={{ fontWeight: 650, color: "var(--ink)" }}>{title}</span>
              </div>
            </div>

            <div className="topbar-right">
              {configStatus ? (
                <Link
                  href="/settings"
                  className={`badge ${configStatus.pendingChanges ? "warning" : "brand"}`}
                  title="Desired configuration generation vs applied revision"
                >
                  <span className={`dot ${configStatus.pendingChanges ? "warn" : "ok"}`} />
                  <span className="mono">Gen #{configStatus.desiredGeneration}</span>
                  <span>{configStatus.pendingChanges ? "Pending Apply" : "Synced"}</span>
                </Link>
              ) : null}

              <button
                type="button"
                onClick={toggleBilingual}
                className="btn btn-secondary btn-sm"
                title="Toggle Bangla hint labels"
              >
                {bilingual ? "EN + বাংলা" : "EN Only"}
              </button>

              <button
                type="button"
                onClick={toggleTheme}
                className="btn btn-secondary btn-icon"
                aria-label={`Switch to ${theme === "dark" ? "light" : "dark"} mode`}
                title={`Switch to ${theme === "dark" ? "light" : "dark"} mode`}
              >
                {theme === "dark" ? <SunIcon size={16} /> : <MoonIcon size={16} />}
              </button>

              {me ? (
                <div className="row" style={{ gap: "0.5rem", paddingLeft: "0.4rem", borderLeft: "1px solid var(--border)" }}>
                  <div style={{ textAlign: "right", lineHeight: 1.2 }}>
                    <div style={{ fontSize: "0.82rem", fontWeight: 650, color: "var(--ink)" }}>
                      {me.user.displayName}
                    </div>
                    <div className="small mono" style={{ fontSize: "0.71rem" }}>
                      {roleMeta?.label} • TTL {formatTimeRemaining(me.expiresAt)}
                    </div>
                  </div>
                  <button
                    type="button"
                    onClick={handleLogout}
                    disabled={loggingOut}
                    className="btn btn-secondary btn-sm"
                    title="Sign out"
                  >
                    <LogOutIcon size={14} />
                    <span>{loggingOut ? "..." : "Sign out"}</span>
                  </button>
                </div>
              ) : (
                <Link href="/" className="btn btn-primary btn-sm">
                  Sign In
                </Link>
              )}
            </div>
          </header>

          <main className="page-content">
            <div className="page-header">
              <div>
                <h1 className="page-title">{title}</h1>
                {subtitle ? <p className="page-subtitle">{subtitle}</p> : null}
              </div>
              {actions ? <div className="row">{actions}</div> : null}
            </div>

            {isUnauthenticated ? (
              <div className="card" style={{ maxWidth: 560, margin: "2rem auto", textAlign: "center" }}>
                <div className="stack-md" style={{ alignItems: "center" }}>
                  <div
                    style={{
                      width: 48,
                      height: 48,
                      borderRadius: 12,
                      background: "var(--warning-soft)",
                      border: "1px solid var(--warning-border)",
                      color: "var(--warning)",
                      display: "grid",
                      placeItems: "center"
                    }}
                  >
                    <LockIcon size={22} />
                  </div>
                  <h2 style={{ fontSize: "1.2rem" }}>Operator Session Required</h2>
                  <p className="small">
                    Your browser does not have an active management session cookie, or the session expired.
                    Authenticate at the Ariba Shield Gateway to access this control plane view.
                  </p>
                  <div className="row">
                    <Link href="/" className="btn btn-primary">
                      Go to Sign In Gateway
                    </Link>
                    <Link href="/setup" className="btn btn-secondary">
                      First-Time Setup
                    </Link>
                  </div>
                </div>
              </div>
            ) : (
              children
            )}
          </main>
        </div>
      </div>
    </ConsoleContext.Provider>
  );
}
