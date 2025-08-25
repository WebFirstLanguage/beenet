mod common;

use bee_core::envelope::BeeEnvelope;
use bee_core::identity::Identity;
use serde_json::json;
use std::time::Duration;

// Test helper
struct TestServer {
    addr: std::net::SocketAddr,
}

async fn start_test_server() -> TestServer {
    use bee_api::server::ApiServer;
    use bee_api::ApiConfig;
    use std::net::TcpListener;

    let listener = TcpListener::bind("127.0.0.1:0").unwrap();
    let addr = listener.local_addr().unwrap();
    drop(listener);

    let config = ApiConfig::new();
    let server = ApiServer::new(config, addr);

    tokio::spawn(async move {
        server.run().await;
    });

    tokio::time::sleep(std::time::Duration::from_millis(100)).await;

    TestServer { addr }
}

#[tokio::test]
async fn test_contract_blackbox_send_receive() {
    // Black-box test: Use only API endpoints, no direct struct access
    let server = start_test_server().await;
    let client = reqwest::Client::new();
    let base_url = format!("http://{}", server.addr);

    // Step 1: Register names
    let response = client
        .post(format!("{}/api/v1/names", base_url))
        .json(&json!({
            "beename": "alice",
            "node_id": "0101010101010101010101010101010101010101010101010101010101010101"
        }))
        .send()
        .await
        .unwrap();
    assert_eq!(response.status(), 201);

    let response = client
        .post(format!("{}/api/v1/names", base_url))
        .json(&json!({
            "beename": "bob",
            "node_id": "0202020202020202020202020202020202020202020202020202020202020202"
        }))
        .send()
        .await
        .unwrap();
    assert_eq!(response.status(), 201);

    // Step 2: Send message from alice to bob
    let response = client
        .post(format!("{}/api/v1/messages", base_url))
        .json(&json!({
            "source": "alice",
            "destination": "bob",
            "payload": base64::Engine::encode(&base64::engine::general_purpose::STANDARD, b"Hello Bob!")
        }))
        .send()
        .await
        .unwrap();
    assert_eq!(response.status(), 201);

    let message_id: String = response.json::<serde_json::Value>().await.unwrap()["id"]
        .as_str()
        .unwrap()
        .to_string();

    // Step 3: Check message status
    let response = client
        .get(format!(
            "{}/api/v1/messages/{}/status",
            base_url, message_id
        ))
        .send()
        .await
        .unwrap();
    assert_eq!(response.status(), 200);

    let status = response.json::<serde_json::Value>().await.unwrap();
    assert!(["accepted", "queued", "sent"].contains(&status["status"].as_str().unwrap()));
}

#[tokio::test]
async fn test_contract_headers_and_digests_present() {
    // Verify that messages have proper headers and digests
    let server = start_test_server().await;
    let client = reqwest::Client::new();
    let base_url = format!("http://{}", server.addr);

    // Register nodes
    client
        .post(format!("{}/api/v1/names", base_url))
        .json(&json!({
            "beename": "sender",
            "node_id": "0303030303030303030303030303030303030303030303030303030303030303"
        }))
        .send()
        .await
        .unwrap();

    client
        .post(format!("{}/api/v1/names", base_url))
        .json(&json!({
            "beename": "receiver",
            "node_id": "0404040404040404040404040404040404040404040404040404040404040404"
        }))
        .send()
        .await
        .unwrap();

    // Send message
    let response = client
        .post(format!("{}/api/v1/messages", base_url))
        .json(&json!({
            "source": "sender",
            "destination": "receiver",
            "payload": base64::Engine::encode(&base64::engine::general_purpose::STANDARD, b"Test message with headers"),
            "include_digest": true
        }))
        .send()
        .await
        .unwrap();

    let message_id: String = response.json::<serde_json::Value>().await.unwrap()["id"]
        .as_str()
        .unwrap()
        .to_string();

    // Get full message details
    let response = client
        .get(format!("{}/api/v1/messages/{}", base_url, message_id))
        .send()
        .await
        .unwrap();

    let message = response.json::<serde_json::Value>().await.unwrap();

    // Verify required fields
    assert!(message["envelope"]["version"].is_number());
    assert!(message["envelope"]["source"].is_string());
    assert!(message["envelope"]["destination"].is_string());
    assert!(message["envelope"]["payload"].is_string());
    assert!(message["envelope"]["digest"].is_string());
}

#[tokio::test]
async fn test_contract_name_resolution_failure() {
    // Contract test: Messages to unresolved names must be rejected
    let server = start_test_server().await;
    let client = reqwest::Client::new();
    let base_url = format!("http://{}", server.addr);

    // Try to send to unregistered name
    let response = client
        .post(format!("{}/api/v1/messages", base_url))
        .json(&json!({
            "source": "unknown-sender",
            "destination": "unknown-receiver",
            "payload": base64::Engine::encode(&base64::engine::general_purpose::STANDARD, b"This should fail")
        }))
        .send()
        .await
        .unwrap();

    assert_eq!(response.status(), 400);

    let error = response.json::<serde_json::Value>().await.unwrap();
    assert_eq!(error["error"], "name_not_resolved");
}

#[tokio::test]
async fn test_contract_part97_compliance_via_api() {
    let server = start_test_server().await;
    let client = reqwest::Client::new();
    let base_url = format!("http://{}", server.addr);

    // Get current Part 97 status
    let response = client
        .get(format!("{}/api/v1/admin/part97", base_url))
        .send()
        .await
        .unwrap();

    let part97 = response.json::<serde_json::Value>().await.unwrap();

    // For radio profiles, Part 97 should be enabled by default
    if part97["profile"] == "radio" {
        assert_eq!(part97["enabled"], true);
        assert_eq!(part97["encryption_allowed"], false);
        assert_eq!(part97["callsign_required"], true);
    }
}

#[tokio::test]
async fn test_contract_message_lifecycle_via_api() {
    let server = start_test_server().await;
    let client = reqwest::Client::new();
    let base_url = format!("http://{}", server.addr);

    // Register nodes
    client
        .post(format!("{}/api/v1/names", base_url))
        .json(&json!({
            "beename": "node-a",
            "node_id": "0505050505050505050505050505050505050505050505050505050505050505"
        }))
        .send()
        .await
        .unwrap();

    client
        .post(format!("{}/api/v1/names", base_url))
        .json(&json!({
            "beename": "node-b",
            "node_id": "0606060606060606060606060606060606060606060606060606060606060606"
        }))
        .send()
        .await
        .unwrap();

    // Send message
    let response = client
        .post(format!("{}/api/v1/messages", base_url))
        .json(&json!({
            "source": "node-a",
            "destination": "node-b",
            "payload": base64::Engine::encode(&base64::engine::general_purpose::STANDARD, b"Lifecycle test")
        }))
        .send()
        .await
        .unwrap();

    let message_id: String = response.json::<serde_json::Value>().await.unwrap()["id"]
        .as_str()
        .unwrap()
        .to_string();

    // Track status transitions
    let mut seen_statuses = Vec::new();

    for _ in 0..10 {
        let response = client
            .get(format!(
                "{}/api/v1/messages/{}/status",
                base_url, message_id
            ))
            .send()
            .await
            .unwrap();

        let status = response.json::<serde_json::Value>().await.unwrap();
        let current_status = status["status"].as_str().unwrap();

        if seen_statuses.is_empty() || seen_statuses.last().unwrap() != current_status {
            seen_statuses.push(current_status.to_string());
        }

        if ["delivered", "failed", "expired"].contains(&current_status) {
            break;
        }

        tokio::time::sleep(Duration::from_millis(100)).await;
    }

    // Verify linear progression
    let valid_progressions = [
        vec!["accepted", "queued", "sent", "delivered"],
        vec!["accepted", "queued", "sent", "failed"],
        vec!["accepted", "queued", "sent", "expired"],
    ];

    let is_valid = valid_progressions.iter().any(|progression| {
        seen_statuses
            .iter()
            .zip(progression.iter())
            .all(|(seen, expected)| seen == expected)
    });

    assert!(is_valid, "Invalid status progression: {:?}", seen_statuses);
}

#[tokio::test]
async fn test_contract_admin_config_via_api() {
    let server = start_test_server().await;
    let client = reqwest::Client::new();
    let base_url = format!("http://{}", server.addr);

    // Get current config
    let response = client
        .get(format!("{}/api/v1/admin/config", base_url))
        .send()
        .await
        .unwrap();
    assert_eq!(response.status(), 200);

    // Update config
    let response = client
        .patch(format!("{}/api/v1/admin/config", base_url))
        .json(&json!({
            "regulatory_mode": "part97_disabled",
            "encryption_enabled": true
        }))
        .send()
        .await
        .unwrap();
    assert_eq!(response.status(), 200);

    // Verify changes
    let response = client
        .get(format!("{}/api/v1/admin/config", base_url))
        .send()
        .await
        .unwrap();

    let config = response.json::<serde_json::Value>().await.unwrap();
    assert_eq!(config["regulatory_mode"], "part97_disabled");
    assert_eq!(config["encryption_enabled"], true);
}

#[tokio::test]
async fn test_contract_signature_verification_via_api() {
    let server = start_test_server().await;
    let client = reqwest::Client::new();
    let base_url = format!("http://{}", server.addr);

    // Create identity for signing
    use ed25519_dalek::SigningKey;
    let signing_key = SigningKey::from_bytes(&[42u8; 32]);
    let identity = Identity::new(signing_key);
    let node_id = identity.node_id();

    // Register node
    client
        .post(format!("{}/api/v1/names", base_url))
        .json(&json!({
            "beename": "signed-node",
            "node_id": hex::encode(node_id.as_bytes()),
            "public_key": hex::encode(identity.public_key().as_bytes())
        }))
        .send()
        .await
        .unwrap();

    // Send signed message
    let payload = b"Signed message";
    let mut envelope = BeeEnvelope::new(node_id, Some(common::test_node_id(1)), payload.to_vec());
    envelope.sign(&identity);

    let response = client
        .post(format!("{}/api/v1/messages", base_url))
        .json(&json!({
            "source": "signed-node",
            "destination": "0707070707070707070707070707070707070707070707070707070707070707",
            "payload": base64::Engine::encode(&base64::engine::general_purpose::STANDARD, payload),
            "signature": hex::encode(envelope.signature.unwrap())
        }))
        .send()
        .await
        .unwrap();

    assert_eq!(response.status(), 201);

    let result = response.json::<serde_json::Value>().await.unwrap();
    assert_eq!(result["signature_valid"], true);
}

#[tokio::test]
async fn test_contract_message_cancellation_via_api() {
    let server = start_test_server().await;
    let client = reqwest::Client::new();
    let base_url = format!("http://{}", server.addr);

    // Register nodes
    client
        .post(format!("{}/api/v1/names", base_url))
        .json(&json!({
            "beename": "cancel-source",
            "node_id": "0808080808080808080808080808080808080808080808080808080808080808"
        }))
        .send()
        .await
        .unwrap();

    client
        .post(format!("{}/api/v1/names", base_url))
        .json(&json!({
            "beename": "cancel-dest",
            "node_id": "0909090909090909090909090909090909090909090909090909090909090909"
        }))
        .send()
        .await
        .unwrap();

    // Send message
    let response = client
        .post(format!("{}/api/v1/messages", base_url))
        .json(&json!({
            "source": "cancel-source",
            "destination": "cancel-dest",
            "payload": base64::Engine::encode(&base64::engine::general_purpose::STANDARD, b"Cancel me")
        }))
        .send()
        .await
        .unwrap();

    let message_id: String = response.json::<serde_json::Value>().await.unwrap()["id"]
        .as_str()
        .unwrap()
        .to_string();

    // Cancel the message while it's queued
    let response = client
        .delete(format!("{}/api/v1/messages/{}", base_url, message_id))
        .send()
        .await
        .unwrap();

    assert_eq!(response.status(), 200);

    // Check status is now failed/cancelled
    let response = client
        .get(format!(
            "{}/api/v1/messages/{}/status",
            base_url, message_id
        ))
        .send()
        .await
        .unwrap();

    let status = response.json::<serde_json::Value>().await.unwrap();
    assert_eq!(status["status"], "failed");
}
