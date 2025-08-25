use ed25519_dalek::{Signature, SigningKey, VerifyingKey};
use serde::{Deserialize, Serialize};
use sha2::{Digest, Sha256};
use std::fmt;

/// Node identifier - SHA-256 hash of public key
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
pub struct NodeId([u8; 32]);

impl NodeId {
    pub fn from_public_key(key: &VerifyingKey) -> Self {
        let mut hasher = Sha256::new();
        hasher.update(key.as_bytes());
        let hash = hasher.finalize();
        let mut id = [0u8; 32];
        id.copy_from_slice(&hash);
        Self(id)
    }

    pub fn as_bytes(&self) -> &[u8; 32] {
        &self.0
    }
}

impl fmt::Display for NodeId {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        for byte in &self.0[..8] {
            write!(f, "{:02x}", byte)?;
        }
        write!(f, "...")
    }
}

/// Node identity with signing capabilities
pub struct Identity {
    signing_key: SigningKey,
    verifying_key: VerifyingKey,
    node_id: NodeId,
}

impl Identity {
    pub fn new(signing_key: SigningKey) -> Self {
        let verifying_key = signing_key.verifying_key();
        let node_id = NodeId::from_public_key(&verifying_key);
        Self {
            signing_key,
            verifying_key,
            node_id,
        }
    }

    pub fn node_id(&self) -> NodeId {
        self.node_id
    }

    pub fn public_key(&self) -> &VerifyingKey {
        &self.verifying_key
    }

    pub fn sign(&self, message: &[u8]) -> Signature {
        use ed25519_dalek::Signer;
        self.signing_key.sign(message)
    }
}
