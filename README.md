# Polyglot URL Shortener (Go + Rust)

A lightweight URL-shortening service built using Go and Rust, designed as a
two-service architecture to explore polyglot backend development.

The Go service acts as the public API and handles URL validation, short-code
generation, and request orchestration. The Rust service acts as the redirect
engine, providing fast URL lookups and click tracking using an in-memory store.

The two services communicate through HTTP/JSON, keeping the boundary between
the API layer and the Rust backend simple and easy to extend.

- **Rust** (`rust-service/`) is the *redirect engine* — the hot path that
  needs to be fast under load. It holds the code→URL mapping and click
  counters in memory (behind an async `RwLock`) and exposes a tiny JSON/HTTP
  API. This is the piece you'd point Redis at in production.
- **Go** (`go-service/`) is the *public API* — generates short codes,
  validates input, and orchestrates calls to the Rust engine. Go's stdlib
  `net/http` and goroutines make this layer quick to build and easy to extend
  (auth, rate limiting, a real database) without touching the Rust core.

```
Client → Go API (:8080) → Rust redirect engine (:9090)
           /shorten            in-memory store
           /r/{code}           (code -> url, code -> clicks)
           /analytics/{code}
```
## Architecture

The application is divided into two independent services:

```text
Client
  |
  v
Go API (:8080)
  |
  | HTTP/JSON
  v
Rust Redirect Engine (:9090)
  |
  v
In-Memory URL Store

## Why two services instead of one language

This isn't "two languages for the sake of it" — it's a small, honest example
of a pattern used in real systems: a orchestration/API layer in a
productivity-focused language, and a performance-critical core in a
systems language, talking over a stable network contract. That's the
story worth telling in an interview, not just "I used Go and Rust."

## Running it

Requirements: Go 1.21+, Rust (stable) + Cargo. No Redis/Postgres needed —
the Rust service keeps state in memory so you can run this in one step.

```bash
./run.sh
```

This builds and starts both services (Rust on `:9090`, Go on `:8080`).
Logs go to `rust.log` and `go.log`.

Or run them manually in two terminals:

```bash
# terminal 1
cd rust-service && cargo run --release

# terminal 2
cd go-service && go run main.go
```

## Try it

```bash
# Create a short link
curl -X POST localhost:8080/shorten -d '{"url":"https://anthropic.com"}'
# -> {"code":"fLh3Fk","short_url":"http://localhost:8080/r/fLh3Fk"}

# Follow it (redirects to the original URL, counts the click)
curl -i localhost:8080/r/fLh3Fk

# Check analytics
curl localhost:8080/analytics/fLh3Fk
# -> {"count":1}
```

## API reference

| Method | Path                | Description                          |
|--------|---------------------|---------------------------------------|
| POST   | `/shorten`           | Body: `{"url": "..."}` → creates a short code |
| GET    | `/r/{code}`          | 302-redirects to the original URL, increments click count |
| GET    | `/analytics/{code}`  | Returns `{"count": N}` total clicks |

Internally, Go calls the Rust service's own small JSON API
(`/set`, `/get/{code}`, `/click/{code}`, `/clicks/{code}`) — see
`rust-service/src/main.rs`.

## Extending it (good "next steps" to mention on your resume/in an interview)

- **Swap in Redis**: replace the `HashMap` in `rust-service/src/main.rs`
  with the `redis` crate — the HTTP handlers don't need to change.
- **Swap in Postgres**: add persistent storage on the Go side for the
  short-code → owner / creation-time metadata.
- **Move to gRPC**: the two services currently talk JSON/HTTP for
  simplicity and zero extra tooling. A natural upgrade is to define
  `proto/shortener.proto` (already sketched in this repo) and switch to
  gRPC with `tonic` (Rust) and `google.golang.org/grpc` (Go) for typed,
  binary-efficient communication — a good "what I'd do with more time"
  talking point.
- **Load test it**: run `k6` or `hey` against `/r/{code}` and report
  throughput/latency numbers — concrete numbers are worth more on a resume
  than "built a scalable system."

## Suggested resume bullet

> Built a polyglot URL-shortening service with a Go API layer and a Rust
> redirect engine communicating over HTTP/JSON, with click analytics and
> an in-memory store designed to be swapped for Redis/Postgres in
> production.
