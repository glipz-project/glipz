# Production self-hosting

The default `docker-compose.yml` runs an independent Glipz instance on **your own
domain**. This is a single-host reference deployment for Lightsail or another Linux
VPS, not a high-availability cluster. Start with a small audience and measure load.

## Prepare infrastructure

Use a maintained Linux distribution with Docker Engine, Compose v2+, Git and
Python 3. A 4 GB server is a starting point, not a capacity guarantee. Build images
off-server if the host cannot accommodate build memory.

Attach a static public IP, point the domain's A record to it, and publish AAAA only
when IPv6 works end to end. Allow TCP 80/443 and optionally UDP 443; restrict SSH to
administrators. Do not expose PostgreSQL, Redis or 8080. Docker port publishing can
bypass some host firewall rules, so check the actual published ports.

Use DNS-only mode if your DNS provider offers a proxy/CDN. The reference Caddy
configuration is directly internet-facing. An extra proxy requires a separate,
narrowly scoped trusted-proxy configuration.

Configure a real SMTP provider and verify the sender/domain using its SPF/DKIM/
DMARC instructions. For SES, request production access before allowing arbitrary
users to register. Prepare an operator contact, moderation process, reviewed legal
policies, encrypted off-server backup destination and monitoring.

For Lightsail, select a Linux instance in the intended region, attach a static IP,
and change DNS at the current domain provider; transferring the domain to AWS is
not required. Enable snapshots in addition to application backups.

## Clone and configure

```bash
git clone https://github.com/glipz-project/glipz.git
cd glipz
python3 scripts/setup-production.py --domain social.example.com --email admin@example.com
```

Replace the domain/email. The generator creates mode-0600 `.env`, independently
generates database administrator/application passwords, Redis password, JWT secret
and a 32-byte federation seed, and refuses to overwrite an existing file. It never
prints secrets. Back up `.env` encrypted off-server. Do not regenerate the federation
key during updates or change the domain casually after accounts exist.

Edit `.env`: configure `SMTP_HOST`, `SMTP_PORT`, `SMTP_TLS`, `SMTP_USERNAME`,
`SMTP_PASSWORD`, and `SMTP_FROM_EMAIL`. Use `starttls` (usually port 587) or `tls`
(usually 465). Follow the provider's requirements. Single-quote dotenv values
containing `$` or `#`; follow Docker dotenv escaping rules for literal quotes.

Local media is the default and persists in `media_data`. For private S3-compatible
storage, use `GLIPZ_STORAGE_MODE=s3` and configure `S3_*` in `.env.example`. Use
HTTPS endpoints, keep buckets private, and back up/version them separately. Configure
bucket CORS when browser uploads use signed storage URLs. All served media passes
through Glipz authorization; do not add public bucket/CDN routes.

Put operator-reviewed `terms.md`, `privacy.md`, `nsfw-guidelines.md`, and
`law-enforcement.md` under `data/legal-docs`. Locale variants such as `terms.ja.md`
take precedence. Review the starting points in `legal-docs.example/`; they are not
ready-made operator policies. The directory is mounted read-only.

```bash
python3 scripts/check-production.py
docker compose up -d --build --wait --wait-timeout 300
docker compose ps
```

Preflight validates resolved configuration without printing secrets. Ordinary
`docker compose config` output includes passwords; do not share it. Advanced
options are in `.env.reference.example`. An optional variable must be explicitly
mapped into the Compose backend environment; adding it only to `.env` is insufficient.
The application container is intentionally not given the database admin password.

## Network and storage model

Caddy redirects HTTP to HTTPS and manages certificate renewal. Public DNS and ports
80/443 must work for ACME issuance. Its `caddy_data` volume preserves certificate
state. The backend serves Vue and the API on the same origin; leave browser
`VITE_API_URL` unset. Discovery and `@user@domain` use `GLIPZ_DOMAIN`.

Only Caddy publishes host ports. The backend runs as uid/gid 10001 with a read-only
root filesystem and writable media volume. The app owns its PostgreSQL schema for
startup migrations but is not a DB superuser. PostgreSQL and Redis are confined to
an internal Docker bridge; traffic between them and the application is unencrypted
within this single-host trust boundary. A remote/multi-host database must instead
use verified TLS and private network controls.

Caddy overwrites forwarding headers. The backend trusts only Caddy's specific
private IP. If `10.245.92.0/24` overlaps your VPC/host networks, change
`GLIPZ_PROXY_SUBNET`, `GLIPZ_PROXY_IP` and `GLIPZ_BACKEND_PROXY_IP` before startup. The backend has a separate
outbound network for federation, SMTP and storage. Auth/proxy/federation rate limits
fail closed when Redis is unavailable. All services restart unless deliberately
stopped; Docker logs are rotated.

## Bootstrap administrator and verify

The first registrant is **not** automatically an administrator. Register and verify
your operator account, then retrieve its UUID:

```bash
docker compose exec postgres psql -U postgres -d glipz
```

In psql, use your exact registered email:

```sql
SELECT id, handle, email FROM users WHERE email = 'your-verified-address@example.com';
```

Exit with `\q`, add that UUID to `GLIPZ_ADMIN_USER_IDS` in `.env`, then run
`docker compose up -d --no-deps backend`. Verify `/admin`, MFA, registration policy,
moderation and announcements before public launch. Registration is not automatically
closed during bootstrap; restrict ingress during setup if the domain is already busy.

Before launch verify HTTPS/redirect, `/health`, federation discovery, real mailbox
verification, login/logout, posting/replies, private media access, DM/notifications,
admin access and federation with a separate test instance. Verify restart persistence
and an isolated restore. Monitor service health, disk space, mail/federation errors,
certificate renewal and backup age. Containers reporting healthy is not an end-to-end
mail, certificate, or federation test.

## Backup

```bash
sh scripts/backup-production.sh
```

Run from a server checkout with a working `.env`. This creates a private timestamped
`backups/` directory, briefly stops the backend, dumps PostgreSQL, archives local
media/legal documents and saves `.env` and the Git revision. It restarts the backend
on exit even if backup fails. A directory without a successful completion message
is incomplete. Schedule the command and alert on failures; encrypt/copy completed
backups off-server and apply retention. Backups contain keys and private user data.
S3 requires its own backup. Snapshots supplement, not replace, tested app backups.

## Restore to a fresh isolated host

Use the recorded Git revision/image and matching schema. Restore `.env` with mode
0600 and preserve domain/federation identity. Keep public ingress closed until checks
pass. `/secure/backup` below is a completed backup. **The restore replaces the
destination schema: use a new disposable/fresh database, never a live instance.**

```bash
mkdir -p data/legal-docs
cp /secure/backup/environment.env .env
chmod 600 .env
tar -C data -xzf /secure/backup/legal-docs.tar.gz
python3 scripts/check-production.py
docker compose build backend
docker compose up -d --wait postgres redis
docker compose exec -T postgres pg_restore -U glipz -d glipz \
  --clean --if-exists --no-owner --exit-on-error < /secure/backup/database.dump
docker compose run --rm --no-deps -T --entrypoint tar backend \
  -C /app/data/media -xzf - < /secure/backup/media.tar.gz
docker compose up -d --wait --wait-timeout 300
```

Redis cache/replay state and TLS certificates are recreated. Keep federation ingress
closed for at least the 15-minute nonce retention window after restoring. Restoring
an old DB can revive previously revoked sessions/tokens: apply your recovery policy
before reopening. Verify data, mail, private media and federation before DNS cutover.
Never run two writable copies of the same instance identity simultaneously.

## Updates and rollback

Back up first and record the old Git revision/image. Review the new revision, set
an immutable local tag such as `GLIPZ_IMAGE=glipz:git-<sha>`, run preflight, build
and start the stack. Keep the previous image/backup until checks pass. Migrations
run at startup; rollback may require restoring the matching DB/media backup, not
just switching to an older image. Upstream image tags follow major release channels;
for frozen releases, record reviewed digests, and periodically rebuild/scan updates.

Use `docker compose stop` to stop services. **Do not use `down -v` to update or stop:
it deletes persistent volumes.**

## Existing development checkouts

The old default development stack is now `docker-compose.dev.yml`, using `.env.dev`
and the separate `glipz-dev` project. Production defaults to `glipz-production` so
old dev volumes are not reused. Do not overwrite an existing `.env` or import dev
accounts/keys into production. To retain an old dev volume/project name, explicitly
use `-p <old-project-name>` with the development file and development env file.
See [SETUP.md](SETUP.md). Both Compose files are standalone; never merge them.

## References

- [Docker Compose](https://docs.docker.com/compose/install/)
- [Caddy HTTPS](https://caddyserver.com/docs/automatic-https)
- [Lightsail static IP](https://docs.aws.amazon.com/lightsail/latest/userguide/lightsail-create-static-ip.html)
- [SES production access](https://docs.aws.amazon.com/ses/latest/dg/request-production-access.html)

## Reference validation (2026-09-19)

The configuration was started against fresh isolated Docker volumes. Verified:
non-superuser application role, startup migrations, read-only application root,
HTTPS through Caddy, the configured discovery domain, and the existing registration,
OAuth revocation, session/PAT and private-media security smoke tests. The backup
script was executed, DB/media probe data was restored into a second fresh project,
and the restored application passed startup/health checks. Tests used an internal
test certificate and a local mail sink; public ACME issuance, real SMTP delivery,
DNS and AWS infrastructure must be checked by each operator on their own server.

`python3 -m unittest discover -s scripts/tests -v` checks configuration generation,
overwrite/injection rejection and unsafe Compose variants. CI also validates Caddy
configuration and shell syntax. Development and production data were kept separate.
