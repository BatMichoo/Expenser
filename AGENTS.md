# Project Architecture: Expenser

This document provides an overview of the Expenser codebase architecture, intended for agents and developers to quickly understand the system's design and how its components interact.

## 1. High-Level Architecture

Expenser is a **monolithic web application** following a traditional MVC-like structure but optimized for **Server-Side Rendering (SSR)** with enhanced interactivity via **HTMX**.

- **Backend**: Go (Gin framework).
- **Frontend**: Go HTML Templates + HTMX + Vanilla CSS.
- **Persistence**: PostgreSQL.
- **Hosting**: an app of the home server (github.com/BatMichoo/homeserver): its nginx, login and theme.

## 2. Technology Stack

| Layer                        | Technology                                                       |
| :--------------------------- | :--------------------------------------------------------------- |
| **Language**                 | Go (1.20+)                                                       |
| **Web Framework**            | [Gin](https://gin-gonic.com/)                                    |
| **Templates**                | `html/template` (Standard Library)                               |
| **Frontend Interactivity**   | [HTMX](https://htmx.org/)                                        |
| **Database**                 | PostgreSQL                                                       |
| **Database Driver**          | `lib/pq`                                                         |
| **Migrations**               | [Goose](https://github.com/pressly/goose)                        |
| **Authentication**           | The home server's auth service (headers from its nginx)          |
| **Look**                     | The home server's shared theme (github.com/BatMichoo/theme)      |
| **Containerization**         | Docker, Docker Compose                                           |

## 3. Directory Structure

```text
server/
├── cmd/                # Entry point (main.go)
├── internal/
│   ├── config/         # Configuration loading and environment variables
│   ├── db/             # Database connection and Data Access Objects (DAOs)
│   │   └── migrations/ # SQL migration files (Goose)
│   ├── handlers/       # HTTP request handlers (Controllers)
│   ├── middleware/     # Gin middleware (RequireUser, language, same-origin)
│   ├── models/         # Data structures and domain models
│   ├── services/       # Business logic (e.g., RecurringPaymentService)
│   ├── templates/      # SSR HTML templates (Components, Pages, Responses)
│   └── utilities/      # Helper functions (Time formats, template funcs)
├── static/             # Static assets (CSS, JS, HTMX)
│   └── js/             # Small client-side helpers for HTMX
```

## 4. Key Architectural Patterns

### Request/Response Flow (HTMX Integration)

1.  **Client** triggers an event (e.g., form submit, button click) via HTMX (`hx-post`, `hx-get`).
2.  **Gin Router** (`internal/handlers/routes.go`) routes the request to a **Handler**.
3.  **Handler** (`internal/handlers/*.go`):
    - Validates input.
    - Calls the **Database Layer** or **Service Layer**.
    - Renders a **Template Fragment** (found in `internal/templates/responses/` or `components/`).
4.  **HTMX** receives the HTML fragment and swaps it into the DOM based on the specified target (`hx-target`).

### Charting

Charts are **server-rendered inline SVG**; there is no chart library and no chart JavaScript.

- `internal/chart` turns numbers into SVG geometry (bars, lines, bands, axis ticks); `internal/services/trends*.go` builds the view models.
- Templates in `internal/templates/components/trend-charts.html` and `trends.html` draw the `<svg>`; colours come from CSS variables, exact values from `<title>` tooltips.
- Each section's `/<section>/chart` page is a normal HTMX view: tabs and filters fetch a new HTML fragment.

### Database Layer (Repository Pattern)

The application uses a manual repository-like pattern located in `internal/db/`.

- `db.go`: Initializes the connection and runs migrations.
- `user.go`, `car.go`, `home.go`: Contains methods on the `DB` struct for specific entity operations.
- Uses `database/sql` for direct SQL execution, providing full control over queries.

### Authentication

- **The home server logs people in**, not Expenser. Its nginx asks its auth service about every request and passes the person on in `X-User-Id` (UUID) and `X-User-Name` (percent-encoded UTF-8). nginx replaces whatever the browser sent in them, and Expenser is reachable only through nginx, so they are trusted.
- **`middleware.RequireUser`** reads them and answers 401 without them. In `DEV_MODE` a request without them is the dev seed user. `middleware.CurrentUser` / `currentUserID` give the person to handlers.
- **No users table.** Expenses and bills store only the user id. Reminders, sent in the background, need more, so each bill keeps its owner's name and language (`SyncBillOwner`, from the headers on every request) and its address (`notify_email`).
- **Public routes**: only `/snooze/:token` (signed links in reminder emails) and static files. The home server's nginx site for Expenser must let `/snooze/` through without a login.
- **Account and password** live in the auth service (the header's Account and Log out go there).

### Look (shared theme)

- Full pages are `theme/page` from github.com/BatMichoo/theme (`handlers.renderLayout`): header, footer, `#content`, the side sheet `#sheet` (`themeShowSheet`, `themeHideSheet`) and toasts `#toast` (`themeShowToast`, `themeHideToast`). Its CSS and scripts (htmx included) are served at `/static/theme`; Expenser's own at `/static/expenser`.
- `templates/components/layout.html` holds Expenser's parts: its `theme/nav` (the theme's navigation with Expenser's icons), the content wrapper and the end of the body.
- Language is the shared `homeserver_lang` cookie, light/dark the shared `homeserver_theme` cookie.

### Configuration

Managed in `internal/config/config.go`. It reads from environment variables, which are often provided by a `.env` file (managed by Docker Compose in production). Only `internal/config` reads the environment (`LoadConfig(getenv)`); `.env.dev` is loaded by `config.LoadDevEnv()` from the module root when present. Startup fails on missing DB settings or a weak `LINK_SECRET` (under 32 bytes; it signs snooze links).

## 5. Deployment and Operations

- **The home server** builds `server/Dockerfile` and runs Expenser as a submodule, behind its nginx, with its Postgres and its deployer. HTTPS, certificates, backups and deploys are its job.
- **Makefile**: shortcuts for development (`make start`, `make dev-db`, `make test`).
- **docker-compose.dev.yaml**: the development database only.

## 6. Developer Guidelines for Agents

- **Adding a Route**: Update `internal/handlers/routes.go` and create a corresponding handler method.
- **Handler Dependencies**: Handlers never import `internal/db`. Each one declares the narrow store interface it needs (`internal/handlers/deps.go`; services do the same, e.g. `services.MonthStore`), and `handlers.Store` combines them. `main` builds `handlers.Deps` once; `newHandlers` creates every service once and shares it. A new DB method a handler calls goes into its store interface.
- **Modifying Data**: Add/Edit files in `internal/db/` for database logic and `internal/models/` for data structures. Models hold data and domain rules only; text formatting and translation (e.g. `utilities.MetadataDetails`) live in `internal/utilities`.
- **UI Changes**:
  - For full pages: `internal/templates/pages/`.
  - For reusable parts: `internal/templates/components/`.
  - For HTMX dynamic responses: `internal/templates/responses/`.
- **Database Schema**: Add a new SQL file in `internal/db/migrations/` following the Goose naming convention.

## 7. Best Practices

### Go (Backend)

- **Thin Handlers**: Handlers should only handle request parsing, validation, and response rendering. Business logic belongs in `services/`, and data access belongs in `db/`.
- **Explicit Errors**: Always check and handle errors. Log them on the server and return user-friendly messages/templates.
- **Typed Context**: Use `gin.Context` consistently but avoid over-relying on global state.
- **Context Management**: Pass `context.Context` to database calls to ensure timeouts and cancellations are respected.

### HTMX (Frontend)

- **Return Fragments**: Always return the smallest possible HTML fragment needed to update the UI.
- **OOB Swaps**: Use `hx-swap-oob="true"` to update multiple parts of the page in a single response (e.g., updating a total count while adding a new list item).
- **Progressive Enhancement**: Ensure the application still provides some functionality (or a clear error) if JavaScript is disabled, though HTMX is core to this project's interactivity.
- **Trigger Headers**: Use the `HX-Trigger` response header to trigger events on the client side (e.g., closing a modal or refreshing a chart).

## 8. Anti-Patterns to Avoid

### Go (Backend)

- **SQL in Handlers**: Never write raw SQL strings inside `internal/handlers/`. Always use the methods provided in `internal/db/`.
- **Large Templates**: Avoid putting complex logic (loops, deep conditionals) inside `.html` files. Prepare the data in Go first.
- **Ignoring Config**: Never hardcode credentials or environment-specific values. Use the `config` package.

### HTMX (Frontend)

- **JSON Over HTMX**: Don't return JSON and use client-side JS to render it. The server should return HTML.
- **Full Page Refresh for Actions**: Avoid using standard `<a>` tags or `<form>` submits for actions that only update a part of the page. Use `hx-get`, `hx-post`, etc.
- **Excessive Custom JS**: Only use custom JavaScript (in `static/js/`) for things HTMX can't do, like complex client-side animations.
- **Global Selectors**: Avoid using `hx-target="body"`. Be specific with IDs to prevent unexpected UI flashes.
