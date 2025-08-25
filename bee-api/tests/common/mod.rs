use bee_core::identity::NodeId;
use ed25519_dalek::SigningKey;

pub fn test_node_id(byte: u8) -> NodeId {
    let signing_key = SigningKey::from_bytes(&[byte; 32]);
    let verifying_key = signing_key.verifying_key();
    NodeId::from_public_key(&verifying_key)
}