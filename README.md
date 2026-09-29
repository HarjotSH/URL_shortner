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
## Architecture.

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

## Running the Project

### Prerequisites

Make sure the following are installed:

- Go 1.21 or later
- Rust (stable)
- Cargo

The project currently uses an in-memory store, so Redis or PostgreSQL is not required to run it locally.

### Start Both Services

The easiest way to start the application is with the provided `run.sh` script:

```bash
./run.sh

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

## API Reference

The Go service exposes the following public endpoints:

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/shorten` | Creates a short URL from the submitted URL |
| GET | `/r/{code}` | Redirects to the original URL and records a click |
| GET | `/analytics/{code}` | Returns the total number of clicks |

### Create a Short URL

```bash
curl -X POST http://localhost:8080/shorten \
  -H "Content-Type: application/json" \
  -d '{"url":"https://example.com"}'

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
