use crate::{ApiConfig, message::MessageQueue, registry::NameRegistry};
use std::sync::Arc;
use tokio::sync::RwLock;

/// Shared state for the API server
pub struct ApiState {
    pub config: Arc<RwLock<ApiConfig>>,
    pub registry: Arc<RwLock<NameRegistry>>,
    pub queue: Arc<RwLock<MessageQueue>>,
}

impl ApiState {
    pub fn new(config: ApiConfig) -> Self {
        Self {
            config: Arc::new(RwLock::new(config)),
            registry: Arc::new(RwLock::new(NameRegistry::new())),
            queue: Arc::new(RwLock::new(MessageQueue::new())),
        }
    }
}