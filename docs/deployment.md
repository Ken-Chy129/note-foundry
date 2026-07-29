# NoteFoundry v0.1 Deployment

## Required services

The Compose stack runs Caddy, the Next.js web application, the Go API and worker roles, PostgreSQL, and a one-shot migration role. Production backup storage must be an S3-compatible bucket independent of the application host. The `local-s3` profile starts MinIO only for development and recovery drills.

## Configuration

Copy `.env.example` to `.env` and replace every placeholder. Production requires:

- `APP_ENV=production`
- `PUBLIC_URL=https://notes.example.com`
- `SITE_ADDRESS=notes.example.com`
- a long random PostgreSQL password
- a GitHub OAuth application whose callback URL is `https://notes.example.com/auth/github/callback`
- the immutable numeric GitHub User ID of the sole Knowledge Owner
- TLS-enabled S3-compatible storage credentials scoped to the configured backup bucket and prefix
- a long backup passphrase stored separately from both the application host and backup bucket

Do not commit `.env`, database dumps, backup passphrases, or S3 credentials.

## Start and verify

```bash
docker compose up -d --build
docker compose ps
curl -fsS https://notes.example.com/healthz
curl -fsS https://notes.example.com/readyz
```

The `migrate` role applies embedded, idempotent PostgreSQL migrations before the API and worker start. Caddy obtains and renews HTTPS certificates when `SITE_ADDRESS` is a public hostname with working DNS and inbound ports 80 and 443.

For a local stack with MinIO:

```bash
docker compose --profile local-s3 up -d --build
curl -fsS http://localhost:8088/readyz
```

## GitHub owner sign-in

Open `/auth/github/start` on the configured domain. The callback accepts only the configured numeric `GITHUB_OWNER_ID`; all other GitHub accounts receive no owner session. Owner sessions are server-side, expiring records referenced by secure HTTP-only cookies in production.

## Automatic encrypted backups

The worker schedules one durable daily backup Job. A successful Job:

1. creates a PostgreSQL custom-format dump;
2. archives the attachment directory and a manifest;
3. encrypts the archive with age scrypt before upload;
4. uploads it under `<prefix>/daily/YYYY/MM/` and maintains weekly copies and retention.

Check worker and Job state:

```bash
docker compose logs worker
docker compose exec postgres sh -c \
  'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -c "SELECT kind, state, attempts, last_error, finished_at FROM jobs ORDER BY created_at DESC LIMIT 20;"'
```

Monitor the S3 bucket independently and alert when no recent object exists. Keeping the passphrase only on the application host defeats disaster recovery; keep a protected external copy.

## Restore

Restoration replaces the target database contents and attachment directory and therefore requires `-confirm`. First stop all roles that can read or write application data while leaving PostgreSQL available:

```bash
docker compose stop caddy web api worker
docker compose --profile tools run --rm restore
docker compose up -d api worker web caddy
curl -fsS https://notes.example.com/readyz
```

By default the restore command selects the newest daily object. To restore a specific object:

```bash
docker compose --profile tools run --rm restore \
  /usr/local/bin/notefoundry-restore -confirm \
  -object notefoundry/daily/2026/07/notefoundry-2026-07-29.tar.gz.age
```

For a recovery drill, restore into a separate empty PostgreSQL database and empty attachment directory, then compare note, attachment, link, and search-projection counts before considering the backup recoverable. A wrong passphrase must fail before database replacement.

## Upgrade and rollback preparation

1. Confirm the latest encrypted backup exists in the independent bucket.
2. Record the running image versions and keep the previous source revision available.
3. Pull the new revision and run `docker compose up -d --build`.
4. Wait for the migration role to finish and verify `/healthz`, `/readyz`, owner sign-in, public reading, and search.
5. If application rollback is necessary after a forward-only schema change, restore the pre-upgrade backup instead of running ad-hoc reverse SQL.

## v0.1 release gate

Run the Hermes Agent scenario in `docs/product-scope.md` on the production domain using the real GitHub Owner account and independent S3 bucket. Record the deployed revision, backup object key, restore target, and observed results in `docs/v0.1-acceptance.md` or the release record. Local MinIO and a manually inserted session do not satisfy this external release gate.
