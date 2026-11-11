"use client";

import React, { useCallback, useEffect, useState } from "react";
import { AppShell, useConsole } from "../../components/AppShell";
import { EmptyState, ErrorAlert, LoadingRows, SuccessAlert } from "../../components/StateBanner";
import {
  CloseIcon,
  EditIcon,
  EyeIcon,
  EyeOffIcon,
  LockIcon,
  PlusIcon,
  RefreshIcon,
  TrashIcon,
  UsersIcon
} from "../../components/Icons";
import {
  ApiClientError,
  apiMutate,
  apiRequestWithMeta,
  Role,
  User
} from "../../lib/api";
import { roleBadgeMeta } from "../../lib/format";

function UsersContent() {
  const { me, authLoading } = useConsole();

  const [loading, setLoading] = useState(true);
  const [users, setUsers] = useState<User[]>([]);
  const [nextCursor, setNextCursor] = useState<string | null>(null);
  const [forbidden, setForbidden] = useState(false);
  const [error, setError] = useState<unknown>(null);
  const [success, setSuccess] = useState("");

  const [drawerOpen, setDrawerOpen] = useState(false);
  const [editingUser, setEditingUser] = useState<User | null>(null);
  const [editingEtag, setEditingEtag] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [formError, setFormError] = useState<unknown>(null);

  const [username, setUsername] = useState("");
  const [displayName, setDisplayName] = useState("");
  const [role, setRole] = useState<Role>("operator");
  const [enabled, setEnabled] = useState(true);
  const [password, setPassword] = useState("");
  const [showPass, setShowPass] = useState(false);

  const [deleteTarget, setDeleteTarget] = useState<User | null>(null);
  const [deleting, setDeleting] = useState(false);

  const isAdmin = me?.user.role === "admin";

  const loadUsers = useCallback(async (cursor?: string | null) => {
    setLoading(true);
    setError(null);
    setForbidden(false);
    try {
      const query = cursor ? `?limit=50&cursor=${encodeURIComponent(cursor)}` : "?limit=50";
      const res = await apiRequestWithMeta<User[]>(`/users${query}`);
      if (cursor) {
        setUsers((prev) => [...prev, ...res.data]);
      } else {
        setUsers(res.data);
      }
      setNextCursor(res.meta?.nextCursor ?? null);
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
        void loadUsers();
      } else {
        setForbidden(true);
        setLoading(false);
      }
    } else if (!authLoading && !me) {
      setLoading(false);
    }
  }, [authLoading, me, isAdmin, loadUsers]);

  function openCreate() {
    setEditingUser(null);
    setEditingEtag(null);
    setFormError(null);
    setUsername("");
    setDisplayName("");
    setRole("operator");
    setEnabled(true);
    setPassword("");
    setDrawerOpen(true);
  }

  async function openEdit(user: User) {
    setFormError(null);
    setError(null);
    try {
      const fresh = await apiRequestWithMeta<User>(`/users/${user.id}`);
      const u = fresh.data;
      setEditingUser(u);
      setEditingEtag(fresh.etag ?? `"${u.id}-v${u.version}"`);
      setUsername(u.username);
      setDisplayName(u.displayName);
      setRole(u.role);
      setEnabled(u.enabled);
      setPassword("");
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
      if (editingUser) {
        await apiMutate<User>(`/users/${editingUser.id}`, {
          method: "PATCH",
          ifMatch: editingEtag ?? `"${editingUser.id}-v${editingUser.version}"`,
          body: {
            displayName: displayName.trim(),
            role,
            enabled
          }
        });
        setSuccess(`User "${editingUser.username}" updated.`);
      } else {
        await apiMutate<User>("/users", {
          method: "POST",
          body: {
            username: username.trim(),
            displayName: displayName.trim(),
            role,
            password
          }
        });
        setSuccess(`User "${username.trim()}" created with role ${role}.`);
      }

      setDrawerOpen(false);
      await loadUsers();
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
      const fresh = await apiRequestWithMeta<User>(`/users/${deleteTarget.id}`);
      const etag = fresh.etag ?? `"${fresh.data.id}-v${fresh.data.version}"`;
      await apiMutate(`/users/${deleteTarget.id}`, {
        method: "DELETE",
        ifMatch: etag
      });
      setSuccess(`User "${deleteTarget.username}" deleted.`);
      setDeleteTarget(null);
      await loadUsers();
    } catch (err) {
      setError(err);
      setDeleteTarget(null);
    } finally {
      setDeleting(false);
    }
  }

  if (forbidden) {
    return (
      <div className="card" style={{ maxWidth: 580 }}>
        <div className="alert warning" style={{ border: 0, padding: 0, background: "transparent" }}>
          <LockIcon size={20} />
          <div className="stack-sm">
            <strong>Administrator Access Required</strong>
            <p className="small">
              User directory and RBAC management endpoints (<code>/api/v1/users</code>) require the{" "}
              <code>admin</code> role.
            </p>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="stack-md">
      <div className="row-between">
        <div className="row">
          <span className="badge brand">RBAC Enforcement</span>
          <span className="small">
            Role changes or disabling an account automatically revokes its active sessions.
          </span>
        </div>
        <div className="row">
          <button type="button" onClick={() => void loadUsers()} className="btn btn-secondary btn-sm" disabled={loading}>
            <RefreshIcon size={14} />
            <span>Refresh</span>
          </button>
          <button type="button" onClick={openCreate} className="btn btn-primary btn-sm">
            <PlusIcon size={14} />
            <span>Create User</span>
          </button>
        </div>
      </div>

      {success ? <SuccessAlert message={success} onDismiss={() => setSuccess("")} /> : null}
      {error ? <ErrorAlert error={error} onRetry={() => void loadUsers()} /> : null}

      <section className="card card-flush">
        {loading && users.length === 0 ? (
          <LoadingRows count={4} />
        ) : users.length === 0 ? (
          <EmptyState
            icon={<UsersIcon size={20} />}
            title="No users found"
            description="Provision operators or read-only viewers for your team."
          />
        ) : (
          <div className="table-wrap">
            <table className="data-table">
              <thead>
                <tr>
                  <th>Operator</th>
                  <th>Username</th>
                  <th>Role</th>
                  <th>Status</th>
                  <th>Version</th>
                  <th style={{ textAlign: "right" }}>Actions</th>
                </tr>
              </thead>
              <tbody>
                {users.map((user) => {
                  const roleMeta = roleBadgeMeta(user.role);
                  const isSelf = me?.user.id === user.id;
                  return (
                    <tr key={user.id}>
                      <td>
                        <div className="row" style={{ gap: 6 }}>
                          <strong style={{ color: "var(--ink)" }}>{user.displayName}</strong>
                          {isSelf ? <span className="badge brand">You</span> : null}
                        </div>
                        <div className="small mono">{user.id.slice(0, 14)}…</div>
                      </td>
                      <td className="mono">{user.username}</td>
                      <td>
                        <span className={`badge ${roleMeta.tone}`}>{roleMeta.label}</span>
                      </td>
                      <td>
                        <span className={`badge ${user.enabled ? "brand" : "danger"}`}>
                          {user.enabled ? "Active" : "Disabled"}
                        </span>
                      </td>
                      <td className="mono">v{user.version}</td>
                      <td style={{ textAlign: "right" }}>
                        <div className="row" style={{ justifyContent: "flex-end" }}>
                          <button
                            type="button"
                            className="btn btn-secondary btn-sm"
                            onClick={() => void openEdit(user)}
                          >
                            <EditIcon size={13} />
                            <span>Edit</span>
                          </button>
                          <button
                            type="button"
                            className="btn btn-danger btn-sm"
                            onClick={() => setDeleteTarget(user)}
                          >
                            <TrashIcon size={13} />
                            <span>Delete</span>
                          </button>
                        </div>
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
              onClick={() => void loadUsers(nextCursor)}
              disabled={loading}
            >
              Load More Users
            </button>
          </div>
        ) : null}
      </section>

      {/* Create / Edit User Drawer */}
      {drawerOpen ? (
        <div className="backdrop" role="dialog" aria-modal="true" aria-labelledby="user-drawer-title">
          <div className="drawer">
            <div className="drawer-header">
              <div>
                <h2 id="user-drawer-title" style={{ fontSize: "1.1rem" }}>
                  {editingUser ? `Edit User: ${editingUser.username}` : "Create Operator Account"}
                </h2>
                <p className="small">
                  {editingUser
                    ? `Optimistic lock ETag: ${editingEtag}`
                    : "Provision a new admin, operator, or viewer account."}
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

              {!editingUser ? (
                <div className="field">
                  <label className="field-label" htmlFor="user-username">
                    Username
                  </label>
                  <input
                    id="user-username"
                    className="input mono"
                    required
                    value={username}
                    onChange={(e) => setUsername(e.target.value)}
                    placeholder="e.g. sec_operator"
                  />
                  <span className="field-hint">Lowercase letters, digits, underscores, or hyphens.</span>
                </div>
              ) : null}

              <div className="field">
                <label className="field-label" htmlFor="user-display">
                  Display Name
                </label>
                <input
                  id="user-display"
                  className="input"
                  required
                  maxLength={256}
                  value={displayName}
                  onChange={(e) => setDisplayName(e.target.value)}
                  placeholder="Security Operator"
                />
              </div>

              <div className="field">
                <label className="field-label" htmlFor="user-role">
                  Role
                </label>
                <select
                  id="user-role"
                  className="select"
                  value={role}
                  onChange={(e) => setRole(e.target.value as Role)}
                >
                  <option value="admin">Admin — Full control plane, users &amp; proxy settings</option>
                  <option value="operator">Operator — Read &amp; edit websites and WAF policies</option>
                  <option value="viewer">Viewer — Read-only access to configuration &amp; events</option>
                </select>
              </div>

              {editingUser ? (
                <label className="checkbox-row">
                  <input type="checkbox" checked={enabled} onChange={(e) => setEnabled(e.target.checked)} />
                  <div>
                    <div style={{ fontWeight: 600 }}>Account Enabled</div>
                    <div className="small">Disabling revokes all active sessions immediately.</div>
                  </div>
                </label>
              ) : (
                <div className="field">
                  <label className="field-label" htmlFor="user-password">
                    <span>Initial Password</span>
                    <span className="small mono">{password.length} / 12–128 chars</span>
                  </label>
                  <div className="input-wrap">
                    <input
                      id="user-password"
                      className="input"
                      type={showPass ? "text" : "password"}
                      required
                      minLength={12}
                      maxLength={128}
                      value={password}
                      onChange={(e) => setPassword(e.target.value)}
                      placeholder="Minimum 12 characters"
                    />
                    <button
                      type="button"
                      className="input-action"
                      onClick={() => setShowPass((v) => !v)}
                      aria-label={showPass ? "Hide password" : "Show password"}
                    >
                      {showPass ? <EyeOffIcon /> : <EyeIcon />}
                    </button>
                  </div>
                </div>
              )}

              <div className="drawer-footer" style={{ margin: "0 -1.35rem -1.25rem", marginTop: "0.5rem" }}>
                <button type="button" className="btn btn-secondary" onClick={() => setDrawerOpen(false)}>
                  Cancel
                </button>
                <button
                  type="submit"
                  className="btn btn-primary"
                  disabled={saving || (!editingUser && (password.length < 12 || password.length > 128))}
                >
                  {saving ? "Saving..." : editingUser ? "Save User Changes" : "Create User"}
                </button>
              </div>
            </form>
          </div>
        </div>
      ) : null}

      {/* Delete Confirmation Modal */}
      {deleteTarget ? (
        <div className="backdrop backdrop-center" role="dialog" aria-modal="true" aria-labelledby="delete-user-title">
          <div className="modal">
            <div className="modal-header">
              <h2 id="delete-user-title" style={{ fontSize: "1.05rem" }}>
                Delete User Account
              </h2>
              <button type="button" className="btn btn-ghost btn-icon" onClick={() => setDeleteTarget(null)}>
                <CloseIcon />
              </button>
            </div>
            <div className="modal-body stack-sm">
              <p>
                Permanently delete user <strong>{deleteTarget.displayName}</strong> (<code>{deleteTarget.username}</code>)?
              </p>
              <p className="small">
                The last remaining enabled administrator cannot be deleted (<code>409 LAST_ADMIN</code>).
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

export default function UsersPage() {
  return (
    <AppShell
      title="Users & RBAC"
      subtitle="Administer operator accounts, role assignments, account status, and session revocation."
    >
      <UsersContent />
    </AppShell>
  );
}
