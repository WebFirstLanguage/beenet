//! Callsign - Amateur radio callsign validation and regulatory binding
//!
//! Callsigns are uppercase identifiers following the pattern [A-Z0-9/-]{2,16}
//! used for regulatory compliance in amateur radio operations.

use crate::identity::NodeId;
use crate::name::BeeName;
use serde::{Deserialize, Serialize};
use std::fmt;
use std::str::FromStr;
use thiserror::Error;

/// Errors that can occur when validating a Callsign
#[derive(Debug, Error)]
pub enum CallsignError {
    #[error("Callsign must be 2-16 characters, got {0}")]
    InvalidLength(usize),

    #[error("Callsign must contain only uppercase letters, digits, slash, and hyphen")]
    InvalidCharacters,

    #[error("Callsign must be uppercase, found lowercase character")]
    ContainsLowercase,

    #[error("Callsign contains non-ASCII characters")]
    NonAscii,
}

/// A validated amateur radio callsign following the pattern [A-Z0-9/-]{2,16}
#[derive(Debug, Clone, PartialEq, Eq, Hash, Serialize, Deserialize)]
pub struct Callsign(String);

impl Callsign {
    /// Create a new Callsign with validation
    pub fn new(callsign: &str) -> Result<Self, CallsignError> {
        // Check length
        let len = callsign.len();
        if !(2..=16).contains(&len) {
            return Err(CallsignError::InvalidLength(len));
        }

        // Check for non-ASCII
        if !callsign.is_ascii() {
            return Err(CallsignError::NonAscii);
        }

        // Validate each character
        for ch in callsign.chars() {
            match ch {
                'A'..='Z' | '0'..='9' | '/' | '-' => {
                    // Valid character
                }
                'a'..='z' => {
                    return Err(CallsignError::ContainsLowercase);
                }
                _ => {
                    return Err(CallsignError::InvalidCharacters);
                }
            }
        }

        Ok(Callsign(callsign.to_string()))
    }

    /// Get the callsign as a string slice
    pub fn as_str(&self) -> &str {
        &self.0
    }

    /// Parse from a string, normalizing to uppercase
    /// This is more permissive than `new` and will uppercase the input
    pub fn parse_normalized(callsign: &str) -> Result<Self, CallsignError> {
        // First normalize to uppercase
        let normalized = callsign.to_uppercase();
        Self::new(&normalized)
    }
}

impl fmt::Display for Callsign {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{}", self.0)
    }
}

impl FromStr for Callsign {
    type Err = CallsignError;

    fn from_str(s: &str) -> Result<Self, Self::Err> {
        Self::new(s)
    }
}

impl AsRef<str> for Callsign {
    fn as_ref(&self) -> &str {
        &self.0
    }
}

/// Regulatory binding between Callsign, BeeName, and NodeID
/// Used for Part 97 compliance and identity association
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct RegulatoryBinding {
    pub callsign: Callsign,
    pub beename: BeeName,
    pub node_id: NodeId,
}

impl RegulatoryBinding {
    /// Create a new regulatory binding
    pub fn new(callsign: Callsign, beename: BeeName, node_id: NodeId) -> Self {
        Self {
            callsign,
            beename,
            node_id,
        }
    }
}

impl fmt::Display for RegulatoryBinding {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{} ({}) [{}]", self.callsign, self.beename, self.node_id)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use ed25519_dalek::SigningKey;

    #[test]
    fn test_valid_callsigns() {
        assert!(Callsign::new("K7").is_ok());
        assert!(Callsign::new("K7TEST").is_ok());
        assert!(Callsign::new("VE3ABC-9").is_ok());
        assert!(Callsign::new("W1AW/3").is_ok());
        assert!(Callsign::new("2E0XXX").is_ok());
        assert!(Callsign::new(&"A".repeat(16)).is_ok());
    }

    #[test]
    fn test_invalid_callsigns() {
        assert!(Callsign::new("").is_err());
        assert!(Callsign::new("A").is_err());
        assert!(Callsign::new(&"A".repeat(17)).is_err());
        assert!(Callsign::new("k7test").is_err());
        assert!(Callsign::new("K7_TEST").is_err());
        assert!(Callsign::new("K7.TEST").is_err());
        assert!(Callsign::new("K7 TEST").is_err());
    }

    #[test]
    fn test_display() {
        let callsign = Callsign::new("K7TEST").unwrap();
        assert_eq!(callsign.to_string(), "K7TEST");
    }

    #[test]
    fn test_regulatory_binding() {
        let callsign = Callsign::new("K7TEST").unwrap();
        let beename = BeeName::new("test-node").unwrap();
        let signing_key = SigningKey::from_bytes(&[1u8; 32]);
        let node_id = NodeId::from_public_key(&signing_key.verifying_key());

        let binding = RegulatoryBinding::new(callsign, beename, node_id);
        assert_eq!(binding.callsign.as_str(), "K7TEST");
        assert_eq!(binding.beename.as_str(), "test-node");
    }
}
