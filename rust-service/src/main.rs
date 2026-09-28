use axum::{
    extract::{Path, State},
    routing::{get, post},
    Json, Router,
};
use serde::{Deserialize, Serialize};
use std::collections::HashMap;
use std::net::SocketAddr;
use std::sync::Arc;
use tokio::sync::RwLock;

// In-memory store standing in for Redis. Swap this out for a real Redis
// client (the `redis` crate) without touching the HTTP surface at all --
// that's the whole point of putting this behind a service boundary.
#[derive(Default)]
struct Store {
    urls: HashMap<String, String>,
    clicks: HashMap<String, i64>,
}

type SharedStore = Arc<RwLock<Store>>;

#[derive(Deserialize)]
struct SetUrlRequest {
    code: String,
    original_url: String,
}

#[derive(Serialize)]
struct SetUrlResponse {
    success: bool,
}

#[derive(Serialize)]
struct GetUrlResponse {
    original_url: String,
    found: bool,
}

#[derive(Serialize)]
struct ClickResponse {
    success: bool,
}

#[derive(Serialize)]
struct ClicksCountResponse {
    count: i64,
}

async fn set_url(
    State(store): State<SharedStore>,
    Json(req): Json<SetUrlRequest>,
) -> Json<SetUrlResponse> {
    let mut store = store.write().await;
    store.urls.insert(req.code.clone(), req.original_url);
    store.clicks.entry(req.code).or_insert(0);
    Json(SetUrlResponse { success: true })
}

async fn get_url(
    State(store): State<SharedStore>,
    Path(code): Path<String>,
) -> Json<GetUrlResponse> {
    let store = store.read().await;
    match store.urls.get(&code) {
        Some(url) => Json(GetUrlResponse {
            original_url: url.clone(),
            found: true,
        }),
        None => Json(GetUrlResponse {
            original_url: String::new(),
            found: false,
        }),
    }
}

async fn increment_click(
    State(store): State<SharedStore>,
    Path(code): Path<String>,
) -> Json<ClickResponse> {
    let mut store = store.write().await;
    let counter = store.clicks.entry(code).or_insert(0);
    *counter += 1;
    Json(ClickResponse { success: true })
}

async fn get_clicks(
    State(store): State<SharedStore>,
    Path(code): Path<String>,
) -> Json<ClicksCountResponse> {
    let store = store.read().await;
    let count = *store.clicks.get(&code).unwrap_or(&0);
    Json(ClicksCountResponse { count })
}

#[tokio::main]
async fn main() {
    let store: SharedStore = Arc::new(RwLock::new(Store::default()));

    let app = Router::new()
        .route("/set", post(set_url))
        .route("/get/:code", get(get_url))
        .route("/click/:code", post(increment_click))
        .route("/clicks/:code", get(get_clicks))
        .with_state(store);

    let addr = SocketAddr::from(([0, 0, 0, 0], 9090));
    println!("Rust redirect engine listening on {}", addr);

    axum::Server::bind(&addr)
        .serve(app.into_make_service())
        .await
        .unwrap();
}
