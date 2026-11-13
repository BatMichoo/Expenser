# Environment Variables (`.env`)

This project relies on environment variables for configuration, managed through a `.env` file. This keeps sensitive and environment-specific settings separate from your main codebase.

## Setup

1. **Create a `.env` file:** In the root directory of this project, create a new file named `.env`.
2. **Add the variables:** Copy the following content into your new `.env` file. **Replace the placeholder values** with your actual, specific configurations.

## Required Variables

| Variable      | Description                                                                                                           | Example Value     |
| :------------ | :-------------------------------------------------------------------------------------------------------------------- | :---------------- |
| `COUNTRY_ISO` | The ISO 3166-1 alpha-2 country code (e.g., `US` for United States) used during certificate generation.                | `US`              |
| `PROVINCE`    | The province or state (e.g., `California`) used in your certificate's distinguished name.                             | `California`      |
| `CITY`        | The city or locality (e.g., `San Francisco`) used in your certificate's distinguished name.                           | `San Francisco`   |
| `ORG_NAME`    | The organization name (e.g., `MyCompany Inc.`) included in your certificate's distinguished name.                     | `My Company Inc.` |
| `LAN_DOMAIN`  | The local area network domain name (e.g., `mywebapp.lan`) used to access the application within your LAN; also the cookie domain. | `mywebapp.lan`    |
| `LAN_IP`      | The local IP address (e.g., `192.168.1.123`) of the machine where the application is hosted, for Nginx configuration. | `192.168.1.123`   |

## TLS Certificate

nginx's certificate comes from a private certificate authority (CA) for your home, made and used by the `certs` container (`certs/issue-certs.sh`). `docker compose up` runs it before nginx starts. The first time, it creates the CA in the `ca-data` volume. After that, it issues a new certificate for `LAN_DOMAIN` and `LAN_IP` only when the current one is missing, expires within 30 days, or no longer matches them. Certificates last 397 days, and autodeploy restarts nginx when one is renewed.

**Trust the CA once on every device.** Its public certificate is `nginx/certs/rootCA.crt`:

- **iPhone/iPad:** AirDrop or email the file and install the profile (Settings > General > VPN & Device Management). Then turn on full trust under Settings > General > About > Certificate Trust Settings.
- **Android:** Settings > Security > Encryption & credentials > Install a certificate > CA certificate.
- **Desktop:** import it as a trusted authority (Firefox: Settings > Certificates > Authorities > Import).

Renewals don't need this again; only a new CA does.

**Commands:**

- `make certs`: issue a certificate now, if needed.
- `make renew-certs`: issue one now regardless, and restart nginx.
- `make ca-backup` / `make ca-restore`: save the CA to `ca-backup.tgz` and put it back, for example on a new server. Keep the backup private: the CA's key can sign a certificate for any website, and your devices would believe it. If the CA is lost, every device has to trust a new one.

## .env Example

Gin runs in release mode unless `DEV_MODE=true`; never set `DEV_MODE` in production.

### NGINX Config

COUNTRY_ISO=US<br>
PROVINCE=California<br>
CITY=San Francisco<br>
ORG_NAME=MyCompany Inc.<br>
LAN_DOMAIN=mywebapp.lan<br>
LAN_IP=192.168.1.123<br>

### DB Config

DB_USER=admin<br>
DB_PASS=admin<br>
DB_HOST=db<br>
DB_PORT=4000<br>
DB_NAME=expenses<br>
DB_SSLMODE=disable<br>
APP_PORT=8080<br>

Tests use the database `DB_NAME` + `_test` on the same server, created on first run; they read `server/.env.dev`, not this file.

### JWT Config

JWT_SECRET=very_secret_JWT_key_for_amazing_security<br>
JWT_EXPIRATION_HOURS=24<br>

### Reminders

REMINDER_CHECK_INTERVAL=24h<br>
SMTP_HOST=smtp.gmail.com<br>
SMTP_PORT=587<br>
SMTP_USER=you@gmail.com<br>
SMTP_PASSWORD=your-app-password<br>
SMTP_FROM=Expenser &lt;you@gmail.com&gt;<br>

Reminder emails need a Gmail **app password** (Google account → Security → 2-Step Verification → App passwords), not your normal password. Without `SMTP_HOST` reminders are only written to the log.

Receipt PDFs that contain text are read with `pdftotext` (Poppler) and sent to Gemini as text, which works for long receipts printed on one tall page. The Docker image installs it; for `make start`, install `poppler-utils` (or your system's Poppler package). Without it, PDFs are sent as they are.

Set `APP_URL` (e.g. `https://expenser.homeserver.lan`) to give reminder emails an **Open bills** button and, for bills that keep reminding, links to pause reminders for 1 day, 3 days or 1 week. Snooze links are signed, so they work without logging in, only for the reminder they came in, and for 14 days; opening one asks for a confirming click before anything changes. They only work on devices that can reach `APP_URL`, i.e. your home network.

In development, `DEV_MODE=true` (in `server/.env.dev`) adds a **Send test email** button to Settings → Account. It mails a sample reminder to the address typed there, so you can check the SMTP setup straight away. Leave `DEV_MODE` unset in production.

# Auto-deploy

On the server, run once from the repo root:

```sh
make autodeploy
```

This starts a `deployer` container that checks GitHub every 30 minutes. When `main` has a new commit and every CI job for it has passed, it fast-forwards the local checkout and runs `docker compose up -d --build`. It then waits up to 2 minutes for the app to answer `/login`. If it does not, it rolls back to the previous commit, rebuilds, and never retries the bad commit (skipped commits are listed in `.git/autodeploy-skip`). Commits with failing CI are skipped, and nothing is deployed if the local checkout is not on `main` or has diverged from it. The container restarts with Docker, so it keeps running across reboots.

- `make autodeploy-logs` shows what it is doing.
- `make autodeploy-stop` turns it off.
- Re-run `make autodeploy` after changing anything in `deploy/` so the deployer itself is rebuilt.
- Set `AUTODEPLOY_INTERVAL` (seconds) in the environment when running `make autodeploy` to change how often it checks.

Before each deploy it dumps the database to `backups/` in the repo (gzipped, newest 14 kept; `AUTODEPLOY_BACKUP_KEEP` changes that). If the dump fails, it does not deploy and tries again next time. The dumps are on the same disk as the database, so copy them somewhere else now and then. To restore one:

```sh
gunzip -c backups/expenser-<date>-<commit>.sql.gz | docker compose exec -T db sh -c 'psql -U "$POSTGRES_USER" "$POSTGRES_DB"'
```

(into an empty database: drop and recreate it first, or restore into a new one.)

Rollback does not undo database migrations, so a migration that the previous version cannot work with still needs a manual fix, from the backup taken before the deploy if needed.

## Before merging the grocery-units migration

Merging to `main` is a deploy: the deployer pulls `main`, rebuilds, and the app runs pending migrations on startup. The grocery-units migration (`server/internal/db/migrations/20261112120000_add_grocery_units.go`) adds `unit`, `unit_size` and `size_status` to `groceries_expenses` and fills them for existing rows from the product names. A rollback of the app does not undo that, so take a backup first.

1. Pause the deployer so it cannot pick up `main` while you are not watching:

   ```sh
   make autodeploy-stop
   ```

2. Back up the database on the server, from the repo root (the `db` container already has the user and database name in `POSTGRES_USER` / `POSTGRES_DB`, taken from `DB_USER` / `DB_NAME` in `.env`):

   ```sh
   docker compose exec -T db sh -c 'pg_dump -U "$POSTGRES_USER" "$POSTGRES_DB"' > backup-$(date +%Y%m%d).sql
   ```

   Check the file is not empty (`ls -lh backup-*.sql`, `tail -n 10 backup-*.sql` should include `PostgreSQL database dump complete`) and copy it off the server.

3. Merge, then deploy by hand and watch it (or turn the deployer back on with `make autodeploy` and follow `make autodeploy-logs`):

   ```sh
   git pull --ff-only && docker compose up -d --build db web nginx
   docker compose logs -f web
   ```

4. Check a few grocery receipts and Trends → Item, then re-enable the deployer with `make autodeploy` if you deployed by hand.

To restore the backup (this replaces everything in the database): stop the app, recreate the database, load the dump, and start an app version from before the migration.

```sh
docker compose stop web
docker compose exec -T db sh -c 'dropdb -U "$POSTGRES_USER" "$POSTGRES_DB" && createdb -U "$POSTGRES_USER" "$POSTGRES_DB"'
docker compose exec -T db sh -c 'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -v ON_ERROR_STOP=1' < backup-YYYYMMDD.sql
git checkout <commit before the merge> && docker compose up -d --build web
```

The dump includes the `goose_db_version` table, so the restored database is back at the version before the migration.

The deployer has access to the Docker socket, which is equivalent to root on the host. It only ever runs code that is on `main` and has passed CI.
