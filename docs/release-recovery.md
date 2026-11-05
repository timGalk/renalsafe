# Local academic release and recovery

Phase 9 covers delivery of the synthetic clinician application. It does not complete independent clinical review or AI/model experiments. See the evidence report in `release-evidence.md`.

## Installation and migration

Use a versioned checkout and the prerequisites in README (Docker/Compose and Python; Go/Node/Poppler for host development). Run `python3 scripts/bootstrap-local.py`, then `docker compose up -d --build`. Bootstrap refuses to overwrite credentials. The application applies migrations 001–004 and imports only a PENDING real-source seed. Use the staff activation instructions in `accounts.md`; no shared browser password is supplied. Existing installations retain `.env`, `.local/` and the database volume; do not rerun bootstrap or delete the volume to upgrade. Take a verified backup before rebuilding and starting a new release. Changed migration digests cause startup failure.

Schema migration uses the configured database owner. The Compose PostgreSQL account is a superuser and the application currently uses it for runtime too. Read-only application filesystem, dropped container capabilities and unprivileged application UID do not remove database-owner bypass. This is a documented local academic deployment limit; separate least-privilege runtime/migration accounts are not claimed implemented. Do not expose either service beyond loopback.

Dependency locks, migration/source hashes, OpenAPI identity, host tool versions and exact image IDs are captured with:

```sh
python3 scripts/release-manifest.py --output outputs/release-manifest.json
```

The output must not already exist. Build tools/base image tags are not all pinned to immutable upstream revisions; preserve built images (for example with `docker image save`) and the manifest to reproduce a delivered binary. A source rebuild using mutable OS/package repositories is not asserted bit-for-bit reproducible.

## Backup and restore drill

```sh
python3 scripts/recovery.py backup .local/backups/my-release
python3 scripts/recovery.py verify .local/backups/my-release
# With the documented Playwright installation:
python3 scripts/recovery.py verify .local/backups/my-release --browser
```

Backup exports a PostgreSQL repeatable-read snapshot, uses that snapshot for the custom-format dump and canonical SHA-256/count fingerprints of every public table, and writes its manifest last. Source PDFs, extracted text, rules, patients/labs, assessments, authentication state and audit all reside in that database. No live rows are modified. Keep schema and configuration unchanged during backup; configuration files are copied after the database snapshot. A failed/incomplete directory has no valid completion manifest; select a new directory for retry rather than overwriting it.

The bundle includes `.env`, role hashes and private setup keys, plus the database dump containing password/session/reset hashes and synthetic case content. It is private plaintext (0700 directories, 0600 files), **not encrypted**. Keep only trusted bundles; checksums detect corruption, not malicious replacement or authenticity. Encrypt off-host copies using an operator-managed tool/key. Cluster roles, other databases, model weights, source checkout, Docker image archives and off-host storage are not included.

`verify` checks every file checksum before invoking Docker; restores only into unique disposable containers; compares all tables before/after database restart; invalidates restored browser sessions/reset links; starts the selected local application image; reapplies migrations; checks the audit chain; exports and replays every historical assessment. Optional browser acceptance exercises physician/pharmacist/admin workflows against that isolated clone. The clone is removed afterwards, including its anonymous database volume. Source database/configuration remain intact. Verification reports stay inside the private backup directory. A backup alone is not proof of restoration; retain a successful report.

The restore drill requires a valid administrator service key in the backed-up configuration. If it has been revoked, use the privileged recovery procedure below on a separately restored workspace, not by re-enabling a disabled identity. By default the selected test image is `renalsafe-core:latest`, resolved to its content ID; `RENALSAFE_RELEASE_IMAGE` can select an archived release image. The verification report records the actual image tested.

## Actual disaster recovery and rollback

Perform actual recovery in a separate checkout/Compose project or on a replacement host. Do not restore over a running database. Keep the original volume intact until acceptance and explicit operator cutover. The automatic script intentionally has no live-overwrite command.

1. Retrieve the trusted private bundle, source revision and saved application image. Verify checksums with the isolated drill first. Copy `config/environment` to `.env`, `config/users.json` to `.local/users.json`, and `config/tokens.json` to `.local/tokens.json` in the replacement checkout. Protect `.local` and plaintext files; the hashes-only users file needs to be readable by application UID 10001 (the bootstrap uses mode 0644 for this file).
2. Choose a different Compose project and unused loopback ports, e.g. `COMPOSE_PROJECT_NAME=renalsafe-recovered`, `RENALSAFE_DB_PORT=55433`, `RENALSAFE_HTTP_PORT=8081`. Keep these exports for **every** following Compose command. Start only the new `db`, wait until its healthcheck is healthy, then restore:

   ```sh
   docker compose exec -T db pg_restore -U renalsafe -d renalsafe \
     --no-owner --no-acl --exit-on-error < /trusted/private/backup/database.dump
   ```

3. Before exposing the restored application, invalidate authentication artifacts from the backup. This maintenance changes authentication state deliberately; retain an operator incident record. Do not edit historical audit or assessment records:

   ```sh
   docker compose exec -T db psql -X -v ON_ERROR_STOP=1 -U renalsafe -d renalsafe <<'SQL'
   BEGIN;
   DELETE FROM auth_sessions;
   UPDATE app_users SET reset_hash='', reset_expires=NULL;
   COMMIT;
   SQL
   ```

4. Start the pinned compatible application image, with migrations only for the intended version. Verify staff access, audit, historical export/replay and role restrictions before cutover. Issue new activation/reset links if needed. A restore returns role/disable/password state to backup time: reconcile security changes made since that snapshot and rotate compromised setup keys/passwords before using the recovered workspace.
5. Rollback means starting the previous image with its compatible **pre-upgrade restored database**, not applying guessed reverse SQL to current history. Preserve the failed-upgrade volume for diagnosis. Cross-version downgrade compatibility is not assumed.

For total administrator credential loss, a privileged filesystem/Docker operator can prepare a new identity with `python3 scripts/prepare-admin-recovery.py --output .local/recovery-incident`. This writes a new hashes list and private credential file, and changes no running service. Preserve the current hashes file; install the generated `users.json` as `.local/users.json` (readable by the application UID), then recreate the core container so its bind mount is refreshed. Startup provisions the **new** administrator identity with an `ACCOUNT_PROVISIONED` audit event; it does not reactivate old disabled accounts. Use the private credential with the existing staff administration API to reset/create a managed administrator, verify login, then disable the recovery actor and remove its entry from the hashes configuration. Record the operator action and keep secrets out of screenshots/terminal logs. This is privileged operator recovery, not an unauthenticated application endpoint.

## Release checks

```sh
npm run check:types --prefix web
npm run build --prefix web
python3 scripts/test-evaluation.py
python3 scripts/test-recovery.py
python3 scripts/test-integration.py
# Build the local release image before the disposable installation check:
docker compose build core
python3 scripts/check-installation.py
```

The installation check runs the real bootstrap in a separate private directory, migrates a fresh database, verifies assessment/replay and idempotent startup, denies database connections to verify fail-closed behavior, upgrades a populated 001/002 database to 003/004, restores its own fixture backup, and checks changed-migration refusal. All database changes target unique disposable containers; the check never changes the real Compose database. During database outage, readiness returns 503; assessment authentication may return 401 because its identity lookup also needs the database. Neither path returns successful assessment output.
