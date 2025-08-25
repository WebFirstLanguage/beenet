use crate::{ApiClient, ApiConfig};
use bee_core::clock::MockClock;
use serde_json::json;
use std::collections::HashMap;
use std::net::SocketAddr;
use std::sync::Arc;
use std::sync::atomic::{AtomicU64, Ordering};
use tokio::sync::RwLock;
use warp::{Filter, Reply};

static MESSAGE_ID_COUNTER: AtomicU64 = AtomicU64::new(1);

pub struct ApiServer {
    client: ApiClient<MockClock>,
    addr: SocketAddr,
    cancelled_messages: Arc<RwLock<HashMap<String, bool>>>,
    message_statuses: Arc<RwLock<HashMap<String, String>>>,
    name_bindings: Arc<RwLock<HashMap<String, String>>>,
}

impl ApiServer {
    pub fn new(config: ApiConfig, addr: SocketAddr) -> Self {
        Self {
            client: ApiClient::<MockClock>::with_config(config),
            addr,
            cancelled_messages: Arc::new(RwLock::new(HashMap::new())),
            message_statuses: Arc::new(RwLock::new(HashMap::new())),
            name_bindings: Arc::new(RwLock::new(HashMap::new())),
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
        let name_bindings = self.name_bindings.clone();

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
            .and(with_name_bindings(name_bindings.clone()))
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
            .and(with_name_bindings(name_bindings.clone()))
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
            .and(with_name_bindings(name_bindings.clone()))
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

fn with_name_bindings(
    name_bindings: Arc<RwLock<HashMap<String, String>>>,
) -> impl Filter<Extract = (Arc<RwLock<HashMap<String, String>>>,), Error = std::convert::Infallible>
       + Clone {
    warp::any().map(move || name_bindings.clone())
}

async fn handle_send_message(
    body: serde_json::Value,
    _client: ApiClient<MockClock>,
    message_statuses: Arc<RwLock<HashMap<String, String>>>,
    name_bindings: Arc<RwLock<HashMap<String, String>>>,
) -> Result<impl Reply, warp::Rejection> {
    let source = body.get("source").and_then(|s| s.as_str()).unwrap_or("");
    let destination = body.get("destination").and_then(|d| d.as_str()).unwrap_or("");

    // Validate destination: either a registered BeeName or a 64-hex NodeID
    let names = name_bindings.read().await;
    let is_hex_node_id = destination.len() == 64 
        && destination.chars().all(|c| c.is_ascii_hexdigit());
    let dest_ok = is_hex_node_id || names.contains_key(destination);
    let src_ok = names.contains_key(source);
    drop(names);

    if !src_ok || !dest_ok {
        return Ok(warp::reply::with_status(
            warp::reply::json(&json!({"error": "name_not_resolved"})),
            warp::http::StatusCode::BAD_REQUEST,
        ));
    }

    let has_signature = body.get("signature").is_some();
    let message_id = MESSAGE_ID_COUNTER.fetch_add(1, Ordering::SeqCst).to_string();
    let mut response = json!({"id": message_id, "status": "accepted"});

    if has_signature {
        response["signature_valid"] = json!(true);
    }

    message_statuses
        .write()
        .await
        .insert(message_id.clone(), "accepted".to_string());

    let statuses_clone = message_statuses.clone();
    let id_clone = message_id.clone();
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
    body: serde_json::Value,
    _client: ApiClient<MockClock>,
    name_bindings: Arc<RwLock<HashMap<String, String>>>,
) -> Result<impl Reply, warp::Rejection> {
    use bee_core::name::BeeName;
    use std::str::FromStr;

    let name = body.get("beename").and_then(|v| v.as_str()).unwrap_or("");
    let node_id = body.get("node_id").and_then(|v| v.as_str()).unwrap_or("");

    // Enforce BeeName lowercase [a-z0-9-]{3,32}
    if BeeName::from_str(name).is_err() {
        return Ok(warp::reply::with_status(
            warp::reply::json(&json!({"error": "invalid_beename"})),
            warp::http::StatusCode::BAD_REQUEST,
        ));
    }

    // Enforce NodeID as 32-byte hex (64 chars)
    if node_id.len() != 64 || !node_id.chars().all(|c| c.is_ascii_hexdigit()) {
        return Ok(warp::reply::with_status(
            warp::reply::json(&json!({"error": "invalid_node_id"})),
            warp::http::StatusCode::BAD_REQUEST,
        ));
    }

    let mut names = name_bindings.write().await;
    if names.contains_key(name) {
        return Ok(warp::reply::with_status(
            warp::reply::json(&json!({"error": "name_already_taken"})),
            warp::http::StatusCode::CONFLICT,
        ));
    }
    names.insert(name.to_string(), node_id.to_string());
    drop(names);

    Ok(warp::reply::with_status(
        warp::reply::json(&json!({"status": "registered"})),
        warp::http::StatusCode::CREATED,
    ))
}

async fn handle_list_names(_client: ApiClient<MockClock>) -> Result<impl Reply, warp::Rejection> {
    Ok(warp::reply::json(&json!([])))
}

async fn handle_resolve_name(
    name: String,
    _client: ApiClient<MockClock>,
    name_bindings: Arc<RwLock<HashMap<String, String>>>,
) -> Result<Box<dyn Reply>, warp::Rejection> {
    if let Some(node_id) = name_bindings.read().await.get(&name).cloned() {
        Ok(Box::new(warp::reply::json(&json!({ "beename": name, "node_id": node_id }))))
    } else {
        Ok(Box::new(warp::reply::with_status(
            warp::reply::json(&json!({"error": "name_not_found"})),
            warp::http::StatusCode::NOT_FOUND,
        )))
    }
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
