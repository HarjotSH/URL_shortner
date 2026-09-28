#!/usr/bin/env bash
# Builds and runs both services. Rust engine on :9090, Go API on :8080.
set -e

echo "Building Rust redirect engine..."
(cd rust-service && cargo build --release)

echo "Building Go API..."
(cd go-service && go build -o shortener-go-service .)

echo "Starting Rust redirect engine on :9090..."
(cd rust-service && ./target/release/shortener-rust-service &> ../rust.log &)

sleep 1

echo "Starting Go API on :8080..."
(cd go-service && ./shortener-go-service &> ../go.log &)

sleep 1
echo ""
echo "Both services are up."
echo "  Rust engine log: rust.log"
echo "  Go API log:      go.log"
echo ""
echo "Try it:"
echo "  curl -X POST localhost:8080/shorten -d '{\"url\":\"https://anthropic.com\"}'"
