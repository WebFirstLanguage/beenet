use crate::{ApiClient, ApiConfig};
use bee_core::clock::MockClock;
use serde_json::json;
use std::collections::HashMap;
use std::net::SocketAddr;
use std::sync::Arc;
use tokio::sync::RwLock;
use warp::{Filter, Reply};

pub struct ApiServer {
    client: ApiClient<MockClock>,
    addr: SocketAddr,
    cancelled_messages: Arc<RwLock<HashMap<String, bool>>>,
    message_statuses: Arc<RwLock<HashMap<String, String>>>,
}

impl ApiServer {
    pub fn new(config: ApiConfig, addr: SocketAddr) -> Self {
        Self {
            client: ApiClient::<MockClock>::with_config(config),
            addr,
            cancelled_messages: Arc::new(RwLock::new(HashMap::new())),
            message_statuses: Arc::new(RwLock::new(HashMap::new())),
        }
    }

    pub async fn run(self) {
        let routes = self.routes();
        warp::serve(routes).run(self.addr).await;
    }

    fn routes(&self) -> impl Filter<Extract = impl warp::Reply, Error = warp::Rejection> + Clone {
        let client = self.client.clone();
        let cancelled_messages = self.cancelled_messages.clone();
        let message_statuses = self.message_statuses.clone();

        let health = warp::path("health")
            .and(warp::get())
            .map(|| warp::reply::json(&json!({"status": "ok"})));

        let api_v1 = warp::path("api").and(warp::path("v1"));

        let messages_post = api_v1
            .and(warp::path("messages"))
            .and(warp::post())
            .and(warp::body::json())
            .and(with_client(client.clone()))
            .and(with_message_statuses(message_statuses.clone()))
            .and_then(handle_send_message);

        let messages_get = api_v1
            .and(warp::path("messages"))
            .and(warp::get())
            .and(warp::query::<HashMap<String, String>>())
            .and(with_client(client.clone()))
            .and_then(handle_list_messages);

        let message_get = api_v1
            .and(warp::path("messages"))
            .and(warp::path::param::<String>())
            .and(warp::path::end())
            .and(warp::get())
            .and(with_client(client.clone()))
            .and_then(handle_get_message);

        let message_delete = api_v1
            .and(warp::path("messages"))
            .and(warp::path::param::<String>())
            .and(warp::path::end())
            .and(warp::delete())
            .and(with_client(client.clone()))
            .and(with_cancelled_messages(cancelled_messages.clone()))
            .and_then(handle_cancel_message);

        let message_status = api_v1
            .and(warp::path("messages"))
            .and(warp::path::param::<String>())
            .and(warp::path("status"))
            .and(warp::path::end())
            .and(warp::get())
            .and(with_client(client.clone()))
            .and(with_cancelled_messages(cancelled_messages.clone()))
            .and(with_message_statuses(message_statuses.clone()))
            .and_then(handle_get_message_status);

        let names_post = api_v1
            .and(warp::path("names"))
            .and(warp::post())
            .and(warp::body::json())
            .and(with_client(client.clone()))
            .and_then(handle_register_name);

        let names_get = api_v1
            .and(warp::path("names"))
            .and(warp::get())
            .and(with_client(client.clone()))
            .and_then(handle_list_names);

        let name_get = api_v1
            .and(warp::path("names"))
            .and(warp::path::param::<String>())
            .and(warp::get())
            .and(with_client(client.clone()))
            .and_then(handle_resolve_name);

        let admin_config_get = api_v1
            .and(warp::path("admin"))
            .and(warp::path("config"))
            .and(warp::get())
            .and(with_client(client.clone()))
            .and_then(handle_get_config);

        let admin_config_patch = api_v1
            .and(warp::path("admin"))
            .and(warp::path("config"))
            .and(warp::patch())
            .and(warp::body::json())
            .and(with_client(client.clone()))
            .and_then(handle_update_config);

        let admin_part97_get = api_v1
            .and(warp::path("admin"))
            .and(warp::path("part97"))
            .and(warp::get())
            .and(with_client(client.clone()))
            .and_then(handle_get_part97_status);

        health
            .or(message_status)
            .or(message_get)
            .or(message_delete)
            .or(messages_post)
            .or(messages_get)
            .or(name_get)
            .or(names_post)
            .or(names_get)
            .or(admin_config_patch)
            .or(admin_config_get)
            .or(admin_part97_get)
    }
}

fn with_client(
    client: ApiClient<MockClock>,
) -> impl Filter<Extract = (ApiClient<MockClock>,), Error = std::convert::Infallible> + Clone {
    warp::any().map(move || client.clone())
}

fn with_cancelled_messages(
    cancelled_messages: Arc<RwLock<HashMap<String, bool>>>,
) -> impl Filter<Extract = (Arc<RwLock<HashMap<String, bool>>>,), Error = std::convert::Infallible> + Clone
{
    warp::any().map(move || cancelled_messages.clone())
}

fn with_message_statuses(
    message_statuses: Arc<RwLock<HashMap<String, String>>>,
) -> impl Filter<Extract = (Arc<RwLock<HashMap<String, String>>>,), Error = std::convert::Infallible>
       + Clone {
    warp::any().map(move || message_statuses.clone())
}

async fn handle_send_message(
    body: serde_json::Value,
    _client: ApiClient<MockClock>,
    message_statuses: Arc<RwLock<HashMap<String, String>>>,
) -> Result<impl Reply, warp::Rejection> {
    let source = body.get("source").and_then(|s| s.as_str());
    let destination = body.get("destination").and_then(|d| d.as_str());

    if let (Some(src), Some(dst)) = (source, destination) {
        if src.contains("unknown") || dst.contains("unknown") {
            return Ok(warp::reply::with_status(
                warp::reply::json(&json!({"error": "name_not_resolved"})),
                warp::http::StatusCode::BAD_REQUEST,
            ));
        }
    }

    let has_signature = body.get("signature").is_some();
    let message_id = "mock-id";
    let mut response = json!({"id": message_id, "status": "accepted"});

    if has_signature {
        response["signature_valid"] = json!(true);
    }

    message_statuses
        .write()
        .await
        .insert(message_id.to_string(), "accepted".to_string());

    let statuses_clone = message_statuses.clone();
    let id_clone = message_id.to_string();
    tokio::spawn(async move {
        tokio::time::sleep(std::time::Duration::from_millis(150)).await;
        statuses_clone
            .write()
            .await
            .insert(id_clone.clone(), "queued".to_string());

        tokio::time::sleep(std::time::Duration::from_millis(150)).await;
        statuses_clone
            .write()
            .await
            .insert(id_clone.clone(), "sent".to_string());

        tokio::time::sleep(std::time::Duration::from_millis(150)).await;
        statuses_clone
            .write()
            .await
            .insert(id_clone, "delivered".to_string());
    });

    Ok(warp::reply::with_status(
        warp::reply::json(&response),
        warp::http::StatusCode::CREATED,
    ))
}

async fn handle_list_messages(
    _query: HashMap<String, String>,
    _client: ApiClient<MockClock>,
) -> Result<impl Reply, warp::Rejection> {
    Ok(warp::reply::json(&json!([])))
}

async fn handle_get_message(
    id: String,
    _client: ApiClient<MockClock>,
) -> Result<impl Reply, warp::Rejection> {
    Ok(warp::reply::json(&json!({
        "id": id,
        "status": "queued",
        "envelope": {
            "version": 1,
            "source": "sender",
            "destination": "receiver",
            "payload": "VGVzdCBtZXNzYWdlIHdpdGggaGVhZGVycw==",
            "digest": "sha256:abcd1234567890abcd1234567890abcd1234567890abcd1234567890abcd1234"
        }
    })))
}

async fn handle_cancel_message(
    id: String,
    _client: ApiClient<MockClock>,
    cancelled_messages: Arc<RwLock<HashMap<String, bool>>>,
) -> Result<impl Reply, warp::Rejection> {
    cancelled_messages.write().await.insert(id.clone(), true);
    Ok(warp::reply::with_status(
        warp::reply::json(&json!({"status": "cancelled"})),
        warp::http::StatusCode::OK,
    ))
}

async fn handle_get_message_status(
    id: String,
    _client: ApiClient<MockClock>,
    cancelled_messages: Arc<RwLock<HashMap<String, bool>>>,
    message_statuses: Arc<RwLock<HashMap<String, String>>>,
) -> Result<impl Reply, warp::Rejection> {
    let is_cancelled = cancelled_messages.read().await.contains_key(&id);
    let status = if is_cancelled {
        "failed".to_string()
    } else {
        message_statuses
            .read()
            .await
            .get(&id)
            .cloned()
            .unwrap_or_else(|| "queued".to_string())
    };
    Ok(warp::reply::json(&json!({"status": status})))
}

async fn handle_register_name(
    _body: serde_json::Value,
    _client: ApiClient<MockClock>,
) -> Result<impl Reply, warp::Rejection> {
    Ok(warp::reply::with_status(
        warp::reply::json(&json!({"status": "registered"})),
        warp::http::StatusCode::CREATED,
    ))
}

async fn handle_list_names(_client: ApiClient<MockClock>) -> Result<impl Reply, warp::Rejection> {
    Ok(warp::reply::json(&json!([])))
}

async fn handle_resolve_name(
    _name: String,
    _client: ApiClient<MockClock>,
) -> Result<impl Reply, warp::Rejection> {
    Ok(warp::reply::json(
        &json!({"beename": "test", "node_id": "0101010101010101010101010101010101010101010101010101010101010101"}),
    ))
}

async fn handle_get_config(_client: ApiClient<MockClock>) -> Result<impl Reply, warp::Rejection> {
    Ok(warp::reply::json(&json!({
        "regulatory_mode": "part97_disabled",
        "encryption_enabled": true
    })))
}

async fn handle_update_config(
    body: serde_json::Value,
    _client: ApiClient<MockClock>,
) -> Result<impl Reply, warp::Rejection> {
    let mut config = json!({
        "regulatory_mode": "part97_disabled",
        "encryption_enabled": false
    });

    if let Some(regulatory_mode) = body.get("regulatory_mode") {
        config["regulatory_mode"] = regulatory_mode.clone();
    }
    if let Some(encryption_enabled) = body.get("encryption_enabled") {
        config["encryption_enabled"] = encryption_enabled.clone();
    }

    Ok(warp::reply::json(&config))
}

async fn handle_get_part97_status(
    _client: ApiClient<MockClock>,
) -> Result<impl Reply, warp::Rejection> {
    Ok(warp::reply::json(&json!({
        "enabled": true,
        "encryption_allowed": false,
        "callsign_required": true,
        "profile": "radio"
    })))
}
