use crate::identity::{Identity, NodeId};
use ed25519_dalek::{Signature, Verifier, VerifyingKey};
use serde::{Deserialize, Serialize};
use sha2::{Digest, Sha256};

/// Beenet protocol envelope for all messages
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct BeeEnvelope {
    pub version: u8,
    pub source: NodeId,
    pub destination: Option<NodeId>,
    pub payload: Vec<u8>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub digest: Option<[u8; 32]>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub signature: Option<Vec<u8>>,
}

impl BeeEnvelope {
    pub fn new(source: NodeId, destination: Option<NodeId>, payload: Vec<u8>) -> Self {
        Self {
            version: 1,
            source,
            destination,
            payload,
            digest: None,
            signature: None,
        }
    }

    pub fn parse(data: &[u8]) -> Result<Self, EnvelopeError> {
        // Use CBOR for canonical serialization
        if data.is_empty() {
            return Err(EnvelopeError::InvalidFormat);
        }

        // For now, use JSON as a simple implementation
        // In production, this would use CBOR for canonical serialization
        let envelope: Self =
            serde_json::from_slice(data).map_err(|_| EnvelopeError::InvalidFormat)?;

        // Check version
        if envelope.version != 1 {
            return Err(EnvelopeError::UnsupportedVersion(envelope.version));
        }

        Ok(envelope)
    }

    pub fn serialize(&self) -> Vec<u8> {
        // For now, use JSON as a simple implementation
        // In production, this would use CBOR for canonical serialization
        serde_json::to_vec(self).unwrap_or_default()
    }

    /// Compute SHA-256 digest of the envelope contents
    pub fn compute_digest(&self) -> [u8; 32] {
        let mut hasher = Sha256::new();
        hasher.update(&[self.version]);
        hasher.update(self.source.as_bytes());
        if let Some(dest) = &self.destination {
            hasher.update(&[1u8]);
            hasher.update(dest.as_bytes());
        } else {
            hasher.update(&[0u8]);
        }
        hasher.update(&(self.payload.len() as u32).to_be_bytes());
        hasher.update(&self.payload);
        hasher.finalize().into()
    }

    /// Set the digest field
    pub fn set_digest(&mut self, digest: [u8; 32]) {
        self.digest = Some(digest);
    }

    /// Verify the digest matches the computed value
    pub fn verify_digest(&self) -> Result<(), EnvelopeError> {
        if let Some(stored_digest) = self.digest {
            let computed = self.compute_digest();
            if stored_digest == computed {
                Ok(())
            } else {
                Err(EnvelopeError::InvalidDigest)
            }
        } else {
            Ok(()) // No digest to verify
        }
    }

    /// Sign the envelope with an identity
    pub fn sign(&mut self, identity: &Identity) {
        let sig_input = self.compute_signature_input();
        let signature = identity.sign(&sig_input);
        self.signature = Some(signature.to_bytes().to_vec());
    }

    /// Verify the envelope signature
    pub fn verify_signature(&self, public_key: &VerifyingKey) -> Result<(), EnvelopeError> {
        if let Some(sig_bytes) = &self.signature {
            if sig_bytes.len() != 64 {
                return Err(EnvelopeError::InvalidSignature);
            }

            let mut sig_array = [0u8; 64];
            sig_array.copy_from_slice(sig_bytes);
            let signature = Signature::from_bytes(&sig_array);

            let sig_input = self.compute_signature_input();
            public_key
                .verify(&sig_input, &signature)
                .map_err(|_| EnvelopeError::InvalidSignature)
        } else {
            Ok(()) // No signature to verify
        }
    }

    /// Compute the input for signature generation/verification
    fn compute_signature_input(&self) -> Vec<u8> {
        let mut input = Vec::new();
        input.extend_from_slice(b"BeeNet-Envelope-v1");
        input.push(0x00);
        input.push(self.version);
        input.extend_from_slice(self.source.as_bytes());
        if let Some(dest) = &self.destination {
            input.push(1);
            input.extend_from_slice(dest.as_bytes());
        } else {
            input.push(0);
        }
        input.extend_from_slice(&(self.payload.len() as u32).to_be_bytes());
        input.extend_from_slice(&self.payload);
        if let Some(digest) = &self.digest {
            input.extend_from_slice(digest);
        }
        input
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

    #[error("Invalid digest")]
    InvalidDigest,
}
