# Project Architecture: Expenser

This document provides an overview of the Expenser codebase architecture, intended for agents and developers to quickly understand the system's design and how its components interact.

## 1. High-Level Architecture

Expenser is a **monolithic web application** following a traditional MVC-like structure but optimized for **Server-Side Rendering (SSR)** with enhanced interactivity via **HTMX**.

- **Backend**: Go (Gin framework).
- **Frontend**: Go HTML Templates + HTMX + Vanilla CSS.
- **Persistence**: PostgreSQL.
- **Deployment**: Docker + Nginx.

## 2. Technology Stack

| Layer                        | Technology                                                       |
| :--------------------------- | :--------------------------------------------------------------- |
| **Language**                 | Go (1.20+)                                                       |
| **Web Framework**            | [Gin](https://gin-gonic.com/)                                    |
| **Templates**                | `html/template` (Standard Library)                               |
| - **Frontend Interactivity** | [HTMX](https://htmx.org/) + [Chart.js](https://www.chartjs.org/) |
| **Database**                 | PostgreSQL                                                       |
| **Database Driver**          | `lib/pq`                                                         |
| **Migrations**               | [Goose](https://github.com/pressly/goose)                        |
| **Authentication**           | JWT (JSON Web Tokens)                                            |
| **Reverse Proxy**            | Nginx                                                            |
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
│   ├── middleware/     # Gin middleware (e.g., Auth)
│   ├── models/         # Data structures and domain models
│   ├── services/       # Business logic (e.g., AuthService)
│   ├── templates/      # SSR HTML templates (Components, Pages, Responses)
│   └── utilities/      # Helper functions (Time formats, template funcs)
├── static/             # Static assets (CSS, JS, HTMX)
│   └── js/             # Client-side logic for Charts and HTMX
nginx/                  # Nginx configuration and SSL setup
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

The application uses **Chart.js** for data visualization.

- Data is typically fetched via HTMX or provided in the initial template.
- Client-side scripts in `static/js/` (e.g., `car-chart.js`, `house-chart.js`) initialize and update charts.

### Database Layer (Repository Pattern)

The application uses a manual repository-like pattern located in `internal/db/`.

- `db.go`: Initializes the connection and runs migrations.
- `user.go`, `car.go`, `home.go`: Contains methods on the `DB` struct for specific entity operations.
- Uses `database/sql` for direct SQL execution, providing full control over queries.

### Authentication

- **JWT**: Tokens are issued upon login and validated via `internal/middleware/auth.go`.
- **Middleware**: Protected routes (like `/car/*` and `/house/*`) are grouped and use the `AuthMiddleware`.

### Configuration

Managed in `internal/config/config.go`. It reads from environment variables, which are often provided by a `.env` file (managed by Docker Compose in production).

## 5. Deployment and Operations

- **Makefile**: Provides shortcuts for common tasks (building, running, generating certs).
- **Docker Compose**: Orchestrates the `server`, `db`, and `nginx` containers.
- **Nginx**: Handles SSL termination and proxies requests to the Go application.

## 6. Developer Guidelines for Agents

- **Adding a Route**: Update `internal/handlers/routes.go` and create a corresponding handler method.
- **Modifying Data**: Add/Edit files in `internal/db/` for database logic and `internal/models/` for data structures.
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
- **Excessive Custom JS**: Only use custom JavaScript (in `static/js/`) for things HTMX can't do, like initializing third-party libraries (Chart.js) or complex client-side animations.
- **Global Selectors**: Avoid using `hx-target="body"`. Be specific with IDs to prevent unexpected UI flashes.
