//! TDD Contract Test: NodeID_is_stable_for_public_key_and_collision_resistant
//! This test verifies that NodeID generation from public keys is deterministic
//! and collision-resistant using SHA-256 hashing.

use bee_core::identity::{Identity, NodeId};
use ed25519_dalek::SigningKey;
use proptest::prelude::*;
use sha2::{Digest, Sha256};

#[test]
fn nodeid_is_sha256_of_public_key() {
    // Generate a test key
    let signing_key = SigningKey::from_bytes(&[1u8; 32]);
    let verifying_key = signing_key.verifying_key();

    // Calculate expected NodeID
    let mut hasher = Sha256::new();
    hasher.update(verifying_key.as_bytes());
    let expected_hash = hasher.finalize();

    // Get NodeID from implementation
    let node_id = NodeId::from_public_key(&verifying_key);

    // Verify it matches SHA-256 of public key
    assert_eq!(
        node_id.as_bytes(),
        expected_hash.as_slice(),
        "NodeID must be SHA-256 of public key"
    );
}

#[test]
fn nodeid_stable_across_calls() {
    // Same key should always produce same NodeID
    let signing_key = SigningKey::from_bytes(&[42u8; 32]);
    let verifying_key = signing_key.verifying_key();

    let id1 = NodeId::from_public_key(&verifying_key);
    let id2 = NodeId::from_public_key(&verifying_key);
    let id3 = NodeId::from_public_key(&verifying_key);

    assert_eq!(id1, id2, "NodeID must be stable");
    assert_eq!(id2, id3, "NodeID must be stable");
}

#[test]
fn nodeid_different_for_different_keys() {
    // Different keys should produce different NodeIDs
    let key1 = SigningKey::from_bytes(&[1u8; 32]);
    let key2 = SigningKey::from_bytes(&[2u8; 32]);
    let key3 = SigningKey::from_bytes(&[3u8; 32]);

    let id1 = NodeId::from_public_key(&key1.verifying_key());
    let id2 = NodeId::from_public_key(&key2.verifying_key());
    let id3 = NodeId::from_public_key(&key3.verifying_key());

    assert_ne!(id1, id2, "Different keys must produce different NodeIDs");
    assert_ne!(id2, id3, "Different keys must produce different NodeIDs");
    assert_ne!(id1, id3, "Different keys must produce different NodeIDs");
}

#[test]
fn identity_nodeid_matches_public_key_hash() {
    // Identity should correctly derive NodeID from its key
    let signing_key = SigningKey::from_bytes(&[77u8; 32]);
    let identity = Identity::new(signing_key.clone());

    // Calculate expected NodeID
    let mut hasher = Sha256::new();
    hasher.update(signing_key.verifying_key().as_bytes());
    let expected_hash = hasher.finalize();

    // Get NodeID from Identity
    let node_id = identity.node_id();

    assert_eq!(
        node_id.as_bytes(),
        expected_hash.as_slice(),
        "Identity NodeID must match SHA-256 of its public key"
    );
}

#[test]
fn nodeid_serialization_preserves_value() {
    // Test that NodeID serialization/deserialization preserves the value
    let signing_key = SigningKey::from_bytes(&[99u8; 32]);
    let node_id = NodeId::from_public_key(&signing_key.verifying_key());

    // Serialize
    let serialized = serde_json::to_string(&node_id).unwrap();

    // Deserialize
    let deserialized: NodeId = serde_json::from_str(&serialized).unwrap();

    assert_eq!(
        node_id, deserialized,
        "Serialization round-trip must preserve NodeID"
    );
}

// Property-based tests
proptest! {
    #[test]
    fn prop_nodeid_deterministic(
        key_bytes in prop::array::uniform32(any::<u8>())
    ) {
        // NodeID generation must be deterministic
        let signing_key = SigningKey::from_bytes(&key_bytes);
        let verifying_key = signing_key.verifying_key();

        let id1 = NodeId::from_public_key(&verifying_key);
        let id2 = NodeId::from_public_key(&verifying_key);

        prop_assert_eq!(id1, id2, "NodeID must be deterministic for same key");
    }

    #[test]
    fn prop_nodeid_matches_sha256(
        key_bytes in prop::array::uniform32(any::<u8>())
    ) {
        // NodeID must always be SHA-256 of public key
        let signing_key = SigningKey::from_bytes(&key_bytes);
        let verifying_key = signing_key.verifying_key();

        let node_id = NodeId::from_public_key(&verifying_key);

        let mut hasher = Sha256::new();
        hasher.update(verifying_key.as_bytes());
        let expected = hasher.finalize();

        prop_assert_eq!(node_id.as_bytes(), expected.as_slice(),
                       "NodeID must be SHA-256 of public key");
    }

    #[test]
    fn prop_different_keys_different_nodeids(
        key1 in prop::array::uniform32(any::<u8>()),
        key2 in prop::array::uniform32(any::<u8>())
    ) {
        // Different keys should produce different NodeIDs (with high probability)
        prop_assume!(key1 != key2);

        let signing_key1 = SigningKey::from_bytes(&key1);
        let signing_key2 = SigningKey::from_bytes(&key2);

        let id1 = NodeId::from_public_key(&signing_key1.verifying_key());
        let id2 = NodeId::from_public_key(&signing_key2.verifying_key());

        prop_assert_ne!(id1, id2, "Different keys should produce different NodeIDs");
    }

    #[test]
    fn prop_identity_nodeid_consistent(
        key_bytes in prop::array::uniform32(any::<u8>())
    ) {
        // Identity's NodeID must match direct calculation
        let signing_key = SigningKey::from_bytes(&key_bytes);
        let identity = Identity::new(signing_key.clone());

        let expected = NodeId::from_public_key(&signing_key.verifying_key());
        let actual = identity.node_id();

        prop_assert_eq!(actual, expected, "Identity NodeID must be consistent");
    }
}

// Collision resistance statistical test
#[test]
fn nodeid_collision_resistance() {
    use rand::rngs::StdRng;
    use rand::{Rng, SeedableRng};
    use std::collections::HashSet;

    let mut rng = StdRng::seed_from_u64(12345);
    let mut node_ids = HashSet::new();

    // Generate many NodeIDs and check for collisions
    let num_keys = 10_000;
    for _ in 0..num_keys {
        let mut key_bytes = [0u8; 32];
        rng.fill(&mut key_bytes);

        let signing_key = SigningKey::from_bytes(&key_bytes);
        let node_id = NodeId::from_public_key(&signing_key.verifying_key());

        // Check for collision
        let was_new = node_ids.insert(node_id);
        assert!(
            was_new,
            "NodeID collision detected! This should be extremely rare."
        );
    }

    assert_eq!(
        node_ids.len(),
        num_keys,
        "All {} NodeIDs should be unique",
        num_keys
    );
}

// Test with known test vectors
#[test]
fn nodeid_known_test_vectors() {
    // Use a known key and verify the NodeID matches expected SHA-256
    // Key: all zeros
    let signing_key = SigningKey::from_bytes(&[0u8; 32]);
    let verifying_key = signing_key.verifying_key();
    let node_id = NodeId::from_public_key(&verifying_key);

    // The public key for all-zero signing key in Ed25519
    // and its SHA-256 should be deterministic
    let mut hasher = Sha256::new();
    hasher.update(verifying_key.as_bytes());
    let expected = hasher.finalize();

    assert_eq!(
        node_id.as_bytes(),
        expected.as_slice(),
        "NodeID for known key must match expected hash"
    );

    // Verify the actual bytes (this will be consistent across runs)
    println!("NodeID for all-zero key: {:?}", node_id);
}
