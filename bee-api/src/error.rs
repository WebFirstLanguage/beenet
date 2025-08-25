use thiserror::Error;

#[derive(Debug, Error)]
pub enum ApiError {
    #[error("Name not resolved: {0}")]
    NameNotResolved(String),

    #[error("Callsign required for Part 97 operation")]
    CallsignRequired,

    #[error("Encryption not allowed in Part 97 mode")]
    EncryptionNotAllowedInPart97,

    #[error("Encrypted payload detected in Part 97 mode")]
    EncryptedPayloadInPart97,

    #[error("Payload too large (max {max} bytes, got {size})")]
    PayloadTooLarge { max: usize, size: usize },

    #[error("Invalid message status transition")]
    InvalidStatusTransition,

    #[error("Message not found: {0}")]
    MessageNotFound(String),

    #[error("Cannot cancel message in status: {0}")]
    CannotCancelMessage(String),

    #[error("Registry error: {0}")]
    Registry(#[from] crate::registry::RegistryError),

    #[error("Invalid configuration: {0}")]
    InvalidConfiguration(String),

    #[error("Queue full")]
    QueueFull,
}