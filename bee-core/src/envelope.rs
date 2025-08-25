use crate::identity::NodeId;
use serde::{Deserialize, Serialize};

/// Beenet protocol envelope for all messages
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct BeeEnvelope {
    pub version: u8,
    pub source: NodeId,
    pub destination: Option<NodeId>,
    pub payload: Vec<u8>,
}

impl BeeEnvelope {
    pub fn new(source: NodeId, destination: Option<NodeId>, payload: Vec<u8>) -> Self {
        Self {
            version: 1,
            source,
            destination,
            payload,
        }
    }

    pub fn parse(_data: &[u8]) -> Result<Self, EnvelopeError> {
        // Stub implementation - will be properly implemented later
        Err(EnvelopeError::InvalidFormat)
    }

    pub fn serialize(&self) -> Vec<u8> {
        // Stub implementation
        vec![]
    }
}

#[derive(Debug, thiserror::Error)]
pub enum EnvelopeError {
    #[error("Invalid envelope format")]
    InvalidFormat,

    #[error("Unsupported version: {0}")]
    UnsupportedVersion(u8),

    #[error("Invalid signature")]
    InvalidSignature,
}
