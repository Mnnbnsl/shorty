# Shorty — long links, chopped short

A minimal, self-contained URL shortener. Paste a long address, hit **Chop it**, and get a compact
code you can say out loud. Every link outlives a server restart — persistence is built in.

Built with a **pure-Go backend + SQLite** (zero CGO, zero external services) and a hand-rolled
**paper-ticket frontend** with no framework.

---

## Highlights

- **Base62 micro-codes** — each short code is the primary key of your link, encoded into an
  ultra-short alphanumeric string (`id 125` → code `21`).
- **SQLite persistence** — shortened URLs survive restarts. No in-memory flimsiness.
- **No build friction** — `modernc.org/sqlite` is 100% pure Go, so the project compiles on any
  platform without gcc / CGO toolchains.
- **Instant copy + QR** — one-click clipboard and a scannable QR stub for mobile sharing.
- **CORS-ready** — develop the frontend from any static server; the opt-in middleware just works.
- **Token-bucket rate limiting** — per-IP rate limiter on the shorten endpoint to prevent abuse (5 req/s sustained, burst of 10).
- **LRU redirect cache** — in-memory least-recently-used cache (10 k entries) sits in front of SQLite, so hot redirects never touch disk.
- **Distinct, hand-tuned UI** — warm paper, ink type, one poster-red accent, perforation notches,
  a receipt-style recent list, and interaction states for loading, empty, and error.

---

## Tech stack

| Layer        | Choice                                              | Why                                            |
| ------------ | --------------------------------------------------- | ---------------------------------------------- |
| Backend      | [Go](https://go.dev) `net/http` (Go 1.22+ ServeMux) | Zero dependencies, tiny binaries               |
| Database     | [modernc.org/sqlite](https://modernc.org/sqlite)    | Pure-Go driver — no CGO, no gcc                |
| Cache        | [hashicorp/golang-lru/v2](https://github.com/hashicorp/golang-lru) | In-memory LRU for blazing redirects  |
| Frontend     | Vanilla HTML + CSS + JS                             | No framework, no build step, no `node_modules` |
| Fonts        | Bricolage Grotesque + Fragment Mono                 | Characterful display paired with crisp mono    |

---

## Project structure

```text
url-shortner/
├── backend/
│   ├── cmd/server/main.go            # Entrypoint: DB init, cache init, routes, static serving
│   ├── data/                         # SQLite database (auto-created, git-ignored)
│   └── internal/
│       ├── database/db.go            # SQLite connection, schema, CRUD helpers
│       ├── services/url.service.go   # Shortening pipeline + Base62 encoder + cache-backed lookups
│       ├── handlers/url.handler.go   # HTTP handlers (JSON API)
│       ├── middleware/cors.go        # CORS + preflight handling
│       ├── middleware/ratelimit.go   # Token-bucket rate limiter (shorten endpoint)
│       └── cache/cache.go            # LRU redirect cache (hashicorp/golang-lru/v2)
└── frontend/
    ├── index.html                    # Single-page markup
    └── assets/
        ├── css/style.css             # Paper-cutter design system
        └── js/app.js                 # Fetch, copy, QR, keyboard, recents
```

## How shortening works

1. The handler validates and normalizes the URL (auto-prefixing `https://` when a protocol is
   missing).
2. The service inserts the original URL into SQLite and receives the auto-increment `id`.
3. That `id` is Base62-encoded into the short code.
4. The code is persisted back onto the row, and the full short URL is returned.

```text
POST /url/shorten  {"url":"example.com"}
        │  normalizes to  https://example.com
        ▼
INSERT urls (original_url)                 id = 125
        │
        ▼
EncodeBase62(125)                    →     "21"
        │
        ▼
UPDATE urls SET short_code = '21'
        │
        ▼
201 {"success":true,"shortCode":"21","shortUrl":"http://localhost:8080/21", ...}
```

Database schema (`backend/data/urls.db`):

```sql
CREATE TABLE IF NOT EXISTS urls (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    original_url TEXT NOT NULL,
    short_code TEXT UNIQUE,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

## Rate limiting

The `POST /url/shorten` endpoint is protected by a **token-bucket** rate limiter
(`backend/internal/middleware/ratelimit.go`).

| Parameter | Value | Meaning                                      |
| --------- | ----- | -------------------------------------------- |
| Burst     | 10    | Max tokens a single IP can hold at once       |
| Refill    | 5/s   | Tokens replenished per second per IP          |
| Scope     | Per-IP | Tracked via `r.RemoteAddr` (IP-only, no auth) |

When a request exceeds the limit the server responds with **429 Too Many Requests**:

```json
{"success":false,"error":"rate limit exceeded"}
```

A `Retry-After: 1` header is included so well-behaved clients can back off.
The limiter is stateless across restarts (in-memory `sync.Map` of per-IP buckets) and
applies **only** to the shorten route — redirects and static assets are not throttled.

## LRU redirect cache

Redirects (`GET /{code}`) are served from an in-memory **least-recently-used** cache
powered by [`hashicorp/golang-lru/v2`](https://github.com/hashicorp/golang-lru)
(`backend/internal/cache/cache.go`).

| Property    | Value  | Notes                                     |
| ----------- | ------ | ----------------------------------------- |
| Capacity    | 10 000 | Covers the vast majority of hot short codes |
| Key         | short code (e.g. `21`)                      |
| Value       | original URL (e.g. `https://example.com`)  |
| Eviction    | LRU — least-recently-used entry is dropped when full |

**How it works**

1. On a redirect request the service checks the cache first.
2. **Cache hit** → the original URL is returned immediately; SQLite is never touched.
3. **Cache miss** → the URL is read from SQLite and written into the cache for next time.
4. On server restart the cache is cold; the first hit for each code populates it.

Because redirects are far more frequent than shortens, the cache eliminates the vast
majority of database reads and keeps redirect latency sub-millisecond.

---

## API reference

| Method | Route              | Description                              |
| ------ | ------------------ | ---------------------------------------- |
| `POST` | `/url/shorten`     | Create a short link                      |
| `GET`  | `/api/urls/recent` | Last 10 shortened links (newest first)   |
| `GET`  | `/{code}`          | 302 redirect to the original URL         |
| `GET`  | `/`                | Serves the frontend (`index.html`)       |
| `GET`  | `/assets/*`        | Serves static CSS / JS / images          |

### Shorten a URL

```bash
curl -X POST http://localhost:8080/url/shorten \
  -H "Content-Type: application/json" \
  -d '{"url":"https://example.com"}'
```

```json
{
  "success": true,
  "shortCode": "1",
  "shortUrl": "http://localhost:8080/1",
  "originalUrl": "https://example.com"
}
```

### Fetch recent links

```bash
curl http://localhost:8080/api/urls/recent
```

```json
{
  "success": true,
  "data": [
    { "id": 2, "originalUrl": "https://example.com", "shortCode": "2",
      "shortUrl": "http://localhost:8080/2", "createdAt": "2026-09-16T20:20:29Z" }
  ]
}
```

> Error responses use the same shape: `{ "success": false, "error": "message" }` with an HTTP 4xx/5xx.

## Getting started

Requirements: **Go 1.22+** (project targets 1.25).

```bash
# from the repository root
cd backend
go run cmd/server/main.go
```

Then open [http://localhost:8080](http://localhost:8080).

- The database `backend/data/urls.db` is created automatically on first start.
- Set `FRONTEND_DIR` to point at a different frontend folder if you move the repo layout:

  ```bash
  FRONTEND_DIR=/path/to/frontend go run cmd/server/main.go
  ```

### Try it out

1. Paste `google.com` and hit **Chop it** — you'll get `http://localhost:8080/1`.
2. Open that link — it 302s straight to Google.
3. Copy the link, expand its QR stub, and check **Recent shorts**.
4. Restart the server — the link still resolves.

## Frontend notes

- **Recent shorts** lists the latest 2 links for a tidy demo; the backend already returns up to 10
  in `/api/urls/recent`.
- Keyboard: **Enter** to chop, **Esc** to clear the form.
- All user-supplied text (URLs, codes) is rendered with DOM `textContent`, so the UI is
  XSS-safe even when shortening untrusted links.
- `prefers-reduced-motion` is honored — all animation is gated off when the user asks.

## Roadmap ideas

- Click counts / analytics per code
- Custom aliases (user-picked short codes)
- URL expiry and bulk import
- Embeddable share cards

---