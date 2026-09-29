# Shorty v2 — long links, chopped short

A high-throughput, feature-rich URL shortener. Paste a long address, hit **Chop it**, and get a compact code you can say out loud. 

Built with a **pure-Go backend, PostgreSQL 16 (high concurrent write throughput), and Redis 7 (caching + auto-expiring ephemeral links)**, paired with a hand-rolled **paper-ticket frontend**.

---

## Highlights & What's New in v2

- **PostgreSQL 16 Storage** — Replaced SQLite to unlock true concurrent write throughput (MVCC + connection pooling via `pgxpool`).
- **Redis 7 Distributed Cache** — Replaced the in-memory LRU cache with Redis for sub-millisecond redirects and distributed caching.
- **2-Hour Ephemeral Anonymous Links** — Links created without an account automatically vanish after **2 hours** via Redis TTL expiration and PostgreSQL background cleanup.
- **User Authentication (JWT + Bcrypt)** — Sign up and log in to save links permanently to your account and manage your personal links dashboard.
- **Custom Aliases** — Logged-in users can pick custom short codes (e.g., `shorty/my-brand`). Validated against syntax rules and reserved system keywords.
- **Asynchronous Click Analytics Engine** — Redirects enqueue click events onto a non-blocking channel (10k buffer); worker goroutines batch-insert using PostgreSQL `COPY` for zero impact on redirect latency.
- **Rich Analytics Dashboard** — View total clicks, unique visitors, 14-day daily click charts, top referral domains, and device breakdown (desktop, mobile, tablet, bot).
- **Docker Compose** — One-command startup for PostgreSQL and Redis.

---

## Tech Stack

| Layer | Choice | Why |
|---|---|---|
| **Backend** | [Go 1.25+](https://go.dev) (`net/http` Go 1.22+ ServeMux) | Zero bloated frameworks, high concurrency, tiny binaries |
| **Database** | [PostgreSQL 16](https://www.postgresql.org) via [`pgx/v5`](https://github.com/jackc/pgx) | Robust MVCC concurrent writes, connection pool, batch `COPY` |
| **Cache & TTL** | [Redis 7](https://redis.io) via [`go-redis/v9`](https://github.com/redis/go-redis) | Sub-millisecond redirects, automatic 2h ephemeral key TTL |
| **Auth** | [JWT (golang-jwt/jwt/v5)](https://github.com/golang-jwt/jwt) + [Bcrypt](https://pkg.go.dev/golang.org/x/crypto/bcrypt) | Stateless access tokens + rotating refresh tokens in httpOnly cookies |
| **Frontend** | Vanilla HTML + CSS + JS | Hand-tuned paper ticket design system, no node_modules |
| **Fonts** | Bricolage Grotesque + Fragment Mono | Distinct editorial typography |

---

## Project Structure

```text
url-shortner/
├── docker-compose.yml                  # PostgreSQL 16 & Redis 7 services
├── backend/
│   ├── cmd/server/main.go              # Server entrypoint & route registration
│   ├── migrations/                     # SQL migration files
│   │   └── 001_initial_schema.sql      # Tables: users, urls, clicks, refresh_tokens
│   └── internal/
│       ├── config/config.go            # Env configuration (DATABASE_URL, REDIS_URL, etc.)
│       ├── auth/                       # JWT tokens, bcrypt password hashing, middleware
│       │   ├── jwt.go
│       │   ├── password.go
│       │   └── middleware.go
│       ├── validation/alias.go         # Custom alias validation & reserved word blocklist
│       ├── database/                   # PostgreSQL connection pool & repositories
│       │   ├── postgres.go             # pgxpool connection & schema auto-migration
│       │   ├── models.go               # Go structs for DB records & analytics stats
│       │   ├── url.repo.go             # URL CRUD & Base62 sequence retrieval
│       │   ├── user.repo.go            # User account queries
│       │   ├── click.repo.go           # Batch COPY clicks & aggregate statistics
│       │   └── token.repo.go           # Refresh token storage
│       ├── cache/                      # Redis client & cache delegations
│       │   ├── redis.go
│       │   └── cache.go
│       ├── services/                   # Core business logic
│       │   ├── url.service.go          # Shortening, custom alias, resolving, cleanup worker
│       │   ├── user.service.go         # Registration, login, token rotation
│       │   └── analytics.service.go    # Click buffer channel & batch worker
│       ├── handlers/                   # HTTP JSON handlers
│       │   ├── url.handler.go          # Shorten, redirect, recent & user links
│       │   ├── auth.handler.go         # Register, login, refresh, logout, me
│       │   └── analytics.handler.go    # Stats & raw click logs
│       └── middleware/
│           ├── cors.go                 # CORS with Authorization support
│           └── ratelimit.go            # Token-bucket per-IP rate limiter
└── frontend/
    ├── index.html                      # Single-page markup with auth & analytics modals
    └── assets/
        ├── css/style.css               # Paper-ticket design system & analytics charts
        └── js/app.js                   # Client logic (auth, shorten, dashboard, analytics)
```

---

## Database Schema (PostgreSQL)

```sql
-- Users
CREATE TABLE users (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email         CITEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    display_name  TEXT NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- URLs
CREATE TABLE urls (
    id            BIGINT GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY,
    original_url  TEXT NOT NULL,
    short_code    VARCHAR(64) NOT NULL UNIQUE,
    is_custom     BOOLEAN NOT NULL DEFAULT false,
    user_id       UUID REFERENCES users(id) ON DELETE SET NULL,
    is_ephemeral  BOOLEAN NOT NULL DEFAULT false,
    expires_at    TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Clicks (Analytics)
CREATE TABLE clicks (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    url_id      BIGINT NOT NULL REFERENCES urls(id) ON DELETE CASCADE,
    clicked_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    ip_hash     VARCHAR(64),
    user_agent  TEXT,
    referer     TEXT,
    device_type VARCHAR(32)
);

-- Refresh Tokens
CREATE TABLE refresh_tokens (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash VARCHAR(64) NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

---

## How Ephemeral Links & Custom Aliases Work

1. **Anonymous Shortening**:
   - `is_ephemeral = true`, `expires_at = now() + 2 hours`.
   - Redis key `url:{code}` is set with an explicit `EX 7200` (2 hours).
   - After 2 hours, the Redis key expires and DB queries reject the expired link.
   - A background worker (`services.StartCleanupWorker`) purges expired rows every 15 minutes.
2. **Authenticated Shortening**:
   - `is_ephemeral = false`, `expires_at = NULL`, `user_id = <UUID>`.
   - Cached in Redis for 24 hours.
   - Saved permanently in the user's dashboard.
3. **Custom Aliases**:
   - Requires login (anonymous requests are rejected with HTTP 400).
   - Must be 3–30 characters (`a-z`, `A-Z`, `0-9`, `-`, `_`).
   - Reserved words (`api`, `auth`, `admin`, `dashboard`, `stats`, etc.) are blocked.

---

## API Reference

### URL Shortener & Redirect

| Method | Route | Auth | Description |
|---|---|---|---|
| `POST` | `/url/shorten` | Optional | Create short URL. Accepts optional `customCode` if logged in. |
| `GET` | `/{code}` | Public | 302 redirect. Enqueues async click event. |
| `GET` | `/api/urls/recent` | Public | Returns recent public active links. |

### Authentication

| Method | Route | Description |
|---|---|---|
| `POST` | `/api/auth/register` | Register with `email`, `password`, `displayName`. Returns JWT. |
| `POST` | `/api/auth/login` | Log in with `email`, `password`. Sets httpOnly refresh cookie. |
| `POST` | `/api/auth/refresh` | Rotates access token using refresh cookie. |
| `POST` | `/api/auth/logout` | Revokes refresh token and clears cookie. |
| `GET` | `/api/user/me` | Returns current user profile. (Bearer token required) |

### User Dashboard

| Method | Route | Description |
|---|---|---|
| `GET` | `/api/user/urls` | Returns all links created by the user with total click counts. |
| `DELETE` | `/api/user/urls/{code}` | Deletes a user's short link and purges from Redis. |

### Analytics

| Method | Route | Description |
|---|---|---|
| `GET` | `/api/urls/{code}/stats` | Aggregate stats: total clicks, unique visitors, daily breakdown, referrers, devices. |
| `GET` | `/api/urls/{code}/clicks` | Paginated raw click log. |

---

## Getting Started

### 1. Start PostgreSQL and Redis via Docker Compose

```bash
docker compose up -d
```

Verify containers are running:
```bash
docker ps
# shorty-postgres (healthy on 5432)
# shorty-redis (healthy on 6379)
```

### 2. Run the Go Server

```bash
cd backend
go run cmd/server/main.go
```

The server automatically runs initial database migrations on startup and listens at **http://localhost:8080**.

### 3. Open the App

Visit [http://localhost:8080](http://localhost:8080) in your browser:
- Try shortening without logging in — link will show **2h Ephemeral** status.
- Click **Sign up** in the header to create an account.
- Expand **+ custom alias** to create a custom link (e.g. `shorty/my-link`).
- Click **stats** or the click count badge to inspect click analytics!