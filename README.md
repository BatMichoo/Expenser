# Expenser

The home's expense tracker: house bills, the car, groceries (typed in or
read off receipt photos and PDFs), recurring bills with email reminders,
and charts of where the money goes.

It runs as an app of the home server
([homeserver](https://github.com/BatMichoo/homeserver)), as a submodule at
`expenser.<LAN_DOMAIN>`. The home server does everything around it:
HTTPS and the home certificate authority, logging in (its auth service),
the shared look (the [theme](https://github.com/BatMichoo/theme) module),
Postgres, backups and deploys. Expenser only reads its settings from the
environment and trusts the person the home server's nginx sends it.

## How login works

The home server's nginx asks its auth service about every request. People
without a session go to the login page; everyone else reaches Expenser with
two headers:

- `X-User-Id`: their UUID.
- `X-User-Name`: their display name (else username), percent-encoded UTF-8.

nginx replaces whatever the browser sent in those headers, so Expenser
trusts them. That only holds while Expenser is reachable through nginx
alone: no published port. A request without them gets a 401.

Each person gets a row in `users` on their first request. Their name and
language are kept on it for reminder emails, which are sent in the
background. Account, password and logging out are in the auth service; the
header's Account and Log out go there. Expenser keeps only the reminder
settings (Bills → Reminder settings).

The only pages open without login are the snooze links in reminder emails
(`/snooze/...`). They are signed with `LINK_SECRET`, work only for the
reminder they came in, and expire after 14 days. The home server's nginx
site for Expenser has to let `/snooze/` through without a login.

## Settings

| Variable | Default | What it is |
|---|---|---|
| `DB_HOST`, `DB_PORT`, `DB_NAME`, `DB_USER`, `DB_PASS` | port 5432 | The app's Postgres database |
| `DB_SSLMODE` | `disable` | The database is reached over the Docker network |
| `LINK_SECRET` | (required) | Signs snooze links; at least 32 bytes (`openssl rand -hex 32`). `JWT_SECRET`, its old name, still works; keep the old value and links already emailed keep working |
| `LAN_DOMAIN` | | e.g. `homeserver.lan`: the language and theme cookies are set for it, and Account / Log out go to `https://auth.<LAN_DOMAIN>` |
| `HTTPS_PORT` | `443` | Where the home server serves HTTPS (for the auth links) |
| `APP_URL` | | e.g. `https://expenser.homeserver.lan`: reminder emails get an "Open bills" button and snooze links. Empty: emails without links |
| `APP_PORT` | `8080` | Where the app listens |
| `APP_TIMEZONE` | `Europe/Sofia` | Which day a purchase falls on, month boundaries, "today" for reminders |
| `REMINDER_CHECK_INTERVAL` | `24h` | How often reminders are checked (at least `30m`) |
| `SMTP_HOST`, `SMTP_PORT`, `SMTP_USER`, `SMTP_PASSWORD`, `SMTP_FROM` | port 587 | Reminder emails. Without `SMTP_HOST` they are only logged. Gmail needs an app password (Google account → Security → 2-Step Verification → App passwords) |
| `GEMINI_API_KEY` | | Reads receipts and suggests pack sizes |
| `DEV_MODE` | off | Development only: no login needed (you are the dev seed user), Gin's debug mode, and a "Send test email" button in the reminder settings |
| `SEED_DEV_DATA` | off | Development only: two years of sample data for the dev user |

Receipt PDFs that contain text are read with `pdftotext` (Poppler) and sent
to Gemini as text, which works for long receipts printed on one tall page.
The Docker image installs it. For `make start`, install `poppler-utils` (or
your system's Poppler package); without it, PDFs are sent as they are.

## Joining the home server

The home server needs, besides the submodule:

- `expenser` in `db/apps` and `EXPENSER_DB_PASSWORD` in its `.env`, so
  `db-setup` creates the database and role.
- An `expenser` service built from `./expenser/server`, with no published
  port, given the settings above (`DB_HOST: db`, `DB_NAME: expenser`,
  `DB_USER: expenser`, `LAN_DOMAIN`, `HTTPS_PORT`,
  `APP_URL: https://expenser.<LAN_DOMAIN>`, `LINK_SECRET`, SMTP, Gemini).
- An nginx site for `expenser.*` like the one in its README (every location
  includes `require-login.conf`), plus a `/snooze/` location with
  `public.conf`.

Moving the data and users over from the old Expenser server is in the home
server's `docs/moving-from-expenser.md`.

## Development

```sh
cp server/.env.dev.template server/.env.dev   # fill it in; DEV_MODE=true
make dev-db                                    # Postgres in Docker
make start                                     # http://localhost:8080
```

With `DEV_MODE=true` there's no login: without the home server's headers
you are the dev seed user. `SEED_DEV_DATA=true` gives that user sample data.

## Tests

```sh
make test
```

Tests use the database `DB_NAME` + `_test` on the dev server, created on
first run; they read `server/.env.dev`.
