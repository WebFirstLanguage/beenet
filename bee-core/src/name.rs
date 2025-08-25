//! BeeName - Human-friendly node names with strict validation
//!
//! BeeNames are lowercase alphanumeric identifiers with hyphens,
//! following the pattern [a-z0-9-]{3,32}. They are case-insensitive
//! for comparison but stored in normalized lowercase form.

use serde::{Deserialize, Serialize};
use std::fmt;
use std::str::FromStr;
use thiserror::Error;

/// Errors that can occur when validating a BeeName
#[derive(Debug, Error)]
pub enum BeeNameError {
    #[error("BeeName must be 3-32 characters, got {0}")]
    InvalidLength(usize),

    #[error("BeeName must contain only lowercase letters, digits, and hyphens")]
    InvalidCharacters,

    #[error("BeeName must be lowercase, found uppercase character")]
    ContainsUppercase,

    #[error("BeeName contains non-ASCII characters")]
    NonAscii,
}

/// A validated BeeName following the pattern [a-z0-9-]{3,32}
#[derive(Debug, Clone, PartialEq, Eq, Hash, Serialize, Deserialize)]
pub struct BeeName(String);

impl BeeName {
    /// Create a new BeeName with validation
    pub fn new(name: &str) -> Result<Self, BeeNameError> {
        // Check length
        let len = name.len();
        if len < 3 || len > 32 {
            return Err(BeeNameError::InvalidLength(len));
        }

        // Check for non-ASCII
        if !name.is_ascii() {
            return Err(BeeNameError::NonAscii);
        }

        // Validate each character
        for ch in name.chars() {
            match ch {
                'a'..='z' | '0'..='9' | '-' => {
                    // Valid character
                }
                'A'..='Z' => {
                    return Err(BeeNameError::ContainsUppercase);
                }
                _ => {
                    return Err(BeeNameError::InvalidCharacters);
                }
            }
        }

        Ok(BeeName(name.to_string()))
    }

    /// Get the name as a string slice
    pub fn as_str(&self) -> &str {
        &self.0
    }

    /// Parse from a string, normalizing to lowercase
    /// This is more permissive than `new` and will lowercase the input
    pub fn parse_normalized(name: &str) -> Result<Self, BeeNameError> {
        // First normalize to lowercase
        let normalized = name.to_lowercase();
        Self::new(&normalized)
    }
}

impl fmt::Display for BeeName {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{}", self.0)
    }
}

impl FromStr for BeeName {
    type Err = BeeNameError;

    fn from_str(s: &str) -> Result<Self, Self::Err> {
        Self::new(s)
    }
}

impl AsRef<str> for BeeName {
    fn as_ref(&self) -> &str {
        &self.0
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_valid_beenames() {
        assert!(BeeName::new("abc").is_ok());
        assert!(BeeName::new("test-node").is_ok());
        assert!(BeeName::new("node-123").is_ok());
        assert!(BeeName::new("0123456789").is_ok());
        assert!(BeeName::new("---").is_ok());
        assert!(BeeName::new(&"a".repeat(32)).is_ok());
    }

    #[test]
    fn test_invalid_beenames() {
        assert!(BeeName::new("").is_err());
        assert!(BeeName::new("ab").is_err());
        assert!(BeeName::new(&"a".repeat(33)).is_err());
        assert!(BeeName::new("Test").is_err());
        assert!(BeeName::new("test_node").is_err());
        assert!(BeeName::new("test.node").is_err());
        assert!(BeeName::new("test node").is_err());
    }

    #[test]
    fn test_display() {
        let name = BeeName::new("hello-world").unwrap();
        assert_eq!(name.to_string(), "hello-world");
    }
}
