pub mod admin;
pub mod error;
pub mod message;
pub mod registry;
pub mod server;
pub mod state;

// Re-export commonly used types
pub use error::ApiError;
pub use message::{Message, MessageId, MessageQueue, MessageStatus};


use bee_core::clock::{Clock, MockClock};
use bee_core::identity::NodeId;
use bee_core::name::BeeName;
use std::sync::Arc;
use tokio::sync::RwLock;

/// Main API client for interacting with the Beenet local API
pub struct ApiClient<C: Clock = MockClock> {
    config: Arc<RwLock<ApiConfig>>,
    registry: Arc<RwLock<registry::NameRegistry>>,
    queue: Arc<RwLock<message::MessageQueue>>,
    clock: C,
}

impl<C: Clock> ApiClient<C> {
    pub fn new() -> ApiClient<MockClock> {
        ApiClient::<MockClock>::with_config(ApiConfig::new())
    }

    pub fn new_test() -> ApiClient<MockClock> {
        ApiClient::<MockClock>::with_config(ApiConfig::new())
    }

    pub fn with_config(config: ApiConfig) -> ApiClient<MockClock> {
        ApiClient::<MockClock>::with_config_and_clock(config, MockClock::new())
    }

    pub fn with_config_and_clock(config: ApiConfig, clock: C) -> ApiClient<C> {
        ApiClient {
            config: Arc::new(RwLock::new(config)),
            registry: Arc::new(RwLock::new(registry::NameRegistry::new())),
            queue: Arc::new(RwLock::new(message::MessageQueue::new())),
            clock,
        }
    }

    pub async fn send_to_name(
        &self,
        source: NodeId,
        dest_name: &BeeName,
        payload: Vec<u8>,
    ) -> Result<message::MessageId, ApiError> {
        // Resolve name to NodeId
        let registry = self.registry.read().await;
        let dest_node = registry
            .resolve(dest_name)
            .ok_or_else(|| ApiError::NameNotResolved(dest_name.to_string()))?;
        drop(registry);

        // Create and queue message
        let mut message = message::Message::new(&self.clock, source, dest_node, payload);
        message.transition_to(&self.clock, message::MessageStatus::Queued)?;
        
        let id = message.id();
        let mut queue = self.queue.write().await;
        queue.enqueue(message);
        
        Ok(id)
    }

    pub async fn validate_payload_for_part97(&self, payload: &[u8]) -> Result<(), ApiError> {
        let config = self.config.read().await;
        if !config.is_part97_enabled() {
            return Ok(());
        }

        // Check if payload appears to be encrypted
        // Simple heuristic: high entropy or non-printable bytes
        let non_printable_count = payload.iter()
            .filter(|&&b| !(0x20..=0x7E).contains(&b))
            .count();
        
        if non_printable_count > payload.len() / 4 {
            return Err(ApiError::EncryptedPayloadInPart97);
        }

        Ok(())
    }
}

impl Default for ApiClient<MockClock> {
    fn default() -> Self {
        Self::new()
    }
}

/// API configuration
#[derive(Debug, Clone)]
pub struct ApiConfig {
    regulatory_mode: admin::RegulatoryMode,
    encryption_enabled: bool,
    callsign: Option<bee_core::callsign::Callsign>,
    swarm_id: [u8; 8],
    message_timeout: std::time::Duration,
    max_queue_size: usize,
    #[allow(dead_code)]
    is_radio_profile: bool,
}

impl ApiConfig {
    pub fn new() -> Self {
        Self {
            regulatory_mode: admin::RegulatoryMode::Part97Disabled,
            encryption_enabled: false,
            callsign: None,
            swarm_id: [0x00; 8],
            message_timeout: std::time::Duration::from_secs(300),
            max_queue_size: 1000,
            is_radio_profile: false,
        }
    }

    pub fn new_radio_profile() -> Self {
        Self {
            regulatory_mode: admin::RegulatoryMode::Part97Enabled,
            encryption_enabled: false,
            callsign: None,
            swarm_id: [0x00; 8],
            message_timeout: std::time::Duration::from_secs(300),
            max_queue_size: 1000,
            is_radio_profile: true,
        }
    }

    pub fn regulatory_mode(&self) -> admin::RegulatoryMode {
        self.regulatory_mode
    }

    pub fn is_part97_enabled(&self) -> bool {
        self.regulatory_mode == admin::RegulatoryMode::Part97Enabled
    }

    pub fn encryption_allowed(&self) -> bool {
        !self.is_part97_enabled() && self.encryption_enabled
    }

    pub fn callsign_required(&self) -> bool {
        self.is_part97_enabled()
    }
}

impl Default for ApiConfig {
    fn default() -> Self {
        Self::new()
    }
}