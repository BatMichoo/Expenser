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
| `LAN_DOMAIN`  | The local area network domain name (e.g., `mywebapp.lan`) used to access the application within your LAN.             | `mywebapp.lan`    |
| `LAN_IP`      | The local IP address (e.g., `192.168.1.123`) of the machine where the application is hosted, for Nginx configuration. | `192.168.1.123`   |

## .env Example

GIN_MODE=release

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
SERVER_PORT=8000<br>

### DB Test Config

TEST_DB_USER=admin<br>
TEST_DB_PASS=adminPassword!<br>
TEST_DB_NAME=expenses_test<br>
TEST_DB_HOST=localhost<br>
TEST_DB_PORT=5000<br>

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

Rollback does not undo database migrations, so a migration that the previous version cannot work with still needs a manual fix.

The deployer has access to the Docker socket, which is equivalent to root on the host. It only ever runs code that is on `main` and has passed CI.
