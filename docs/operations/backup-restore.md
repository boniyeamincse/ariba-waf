# Backup and Restore

**Status:** Proposed operational contract; backup/restore is not implemented or verified.

## 1. Backup contents

A consistent backup should include matching SQLite state, immutable configuration revisions and active-revision metadata, required certificates/private keys, trust bundles, and local secrets needed to restore service. It should exclude live sessions, enrollment tokens, and historical logs by default. Document exactly which user/configuration data is included before release.

Because private keys and credentials are sensitive, create an encrypted archive using a vetted authenticated-encryption implementation and a user-supplied passphrase. The passphrase is write-only: do not store it in the database, job payloads, logs, or idempotency records. Protect backup files with restrictive permissions and retention limits.

## 2. Consistency

Use SQLite online backup facilities or a stopped consistent snapshot. Copying only the `.db` file while WAL is active is insufficient. Coordinate database, config, and secret snapshots so they represent the same applied revision. Avoid placing unencrypted temporary files on broadly readable storage.

## 3. Restore validation

Before applying a restore, validate:

- Archive encryption, integrity, schema compatibility, and supported product version.
- Entry count, expanded size, safe relative paths, and no symlinks/path traversal.
- Certificate/key references, config checksum, and rendered configuration compatibility.
- Available disk space and ownership/permissions.

Stage decrypted data in restricted temporary storage and delete it automatically on completion/expiry. Never expose a public download URL or bypass authentication with a job token.

## 4. Restore procedure

1. Require Admin role, recent reauthentication, CSRF protection, and explicit confirmation.
2. Serialize restore against config edits/apply/rollback and create a recoverable checkpoint.
3. Quiesce database writers and restore the matched state safely.
4. Validate and test proxy configuration before activation.
5. Verify the restored running revision and readiness.
6. On failure, recover the checkpoint and report recovery status; do not report success if verification fails.
7. On success, revoke all existing sessions and require login with restored accounts.
8. Record actor, timestamps, selected artifact, result, and sanitized recovery details in the audit trail.

## 5. Retention and recovery drills

Limit backup count/size and account for actual free volume space. Do not silently delete all audit history under disk pressure. Alert operators to failed backup, low space, and expired staging files.

Before production support, test backup creation, encrypted download, corrupt/wrong-passphrase rejection, restore to a clean volume, version mismatch, mid-restore crash, checkpoint recovery, session revocation, and post-restore login. Record restore duration and data loss boundaries after measuring them; none are guaranteed yet.
