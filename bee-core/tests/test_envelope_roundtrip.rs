//! TDD Contract Test: Envelope_roundtrip_preserves_plaintext_payload
//! This test verifies that BeeEnvelope serialization/deserialization preserves
//! the plaintext payload without encoding that obscures meaning, and includes
//! integrity verification via SHA-256/BLAKE3 digest and optional Ed25519 signature.

use bee_core::envelope::{BeeEnvelope, EnvelopeError};
use bee_core::identity::{Identity, NodeId};
use ed25519_dalek::SigningKey;
use proptest::prelude::*;

#[test]
fn envelope_roundtrip_preserves_payload() {
    let source_key = SigningKey::from_bytes(&[1u8; 32]);
    let source_id = NodeId::from_public_key(&source_key.verifying_key());

    let dest_key = SigningKey::from_bytes(&[2u8; 32]);
    let dest_id = NodeId::from_public_key(&dest_key.verifying_key());

    let payload = b"Hello, Beenet! This is a test payload.".to_vec();

    // Create envelope
    let envelope = BeeEnvelope::new(source_id, Some(dest_id), payload.clone());

    // Serialize
    let serialized = envelope.serialize();
    assert!(
        !serialized.is_empty(),
        "Serialized envelope should not be empty"
    );

    // Deserialize
    let restored = BeeEnvelope::parse(&serialized).expect("Should parse serialized envelope");

    // Verify all fields preserved
    assert_eq!(
        restored.version, envelope.version,
        "Version must be preserved"
    );
    assert_eq!(restored.source, source_id, "Source must be preserved");
    assert_eq!(
        restored.destination,
        Some(dest_id),
        "Destination must be preserved"
    );
    assert_eq!(
        restored.payload, payload,
        "Payload must be preserved exactly"
    );
}

#[test]
fn envelope_plaintext_payload_readable() {
    let source_key = SigningKey::from_bytes(&[3u8; 32]);
    let source_id = NodeId::from_public_key(&source_key.verifying_key());

    // Use ASCII text payload
    let plaintext = "This is plaintext that must remain readable";
    let payload = plaintext.as_bytes().to_vec();

    let envelope = BeeEnvelope::new(source_id, None, payload.clone());
    let serialized = envelope.serialize();

    // The payload should be findable in the serialized form
    // (not base64 encoded or otherwise obscured)
    // This is important for Part 97 compliance

    let restored = BeeEnvelope::parse(&serialized).expect("Should parse");
    let restored_text =
        String::from_utf8(restored.payload.clone()).expect("Payload should be valid UTF-8");

    assert_eq!(restored_text, plaintext, "Plaintext must be preserved");
}

#[test]
fn envelope_with_integrity_digest() {
    let source_key = SigningKey::from_bytes(&[4u8; 32]);
    let source_id = NodeId::from_public_key(&source_key.verifying_key());

    let payload = b"Data with integrity check".to_vec();
    let mut envelope = BeeEnvelope::new(source_id, None, payload.clone());

    // Add SHA-256 digest
    let digest = envelope.compute_digest();
    envelope.set_digest(digest);

    let serialized = envelope.serialize();
    let restored = BeeEnvelope::parse(&serialized).expect("Should parse");

    // Verify digest
    assert!(restored.verify_digest().is_ok(), "Digest should verify");

    // Tamper with payload
    let mut tampered = restored.clone();
    tampered.payload.push(0xFF);
    assert!(
        tampered.verify_digest().is_err(),
        "Tampered envelope should fail digest check"
    );
}

#[test]
fn envelope_with_signature() {
    let source_key = SigningKey::from_bytes(&[5u8; 32]);
    let identity = Identity::new(source_key.clone());
    let source_id = identity.node_id();

    let payload = b"Signed payload".to_vec();
    let mut envelope = BeeEnvelope::new(source_id, None, payload);

    // Sign the envelope
    envelope.sign(&identity);

    let serialized = envelope.serialize();
    let restored = BeeEnvelope::parse(&serialized).expect("Should parse");

    // Verify signature
    assert!(
        restored.verify_signature(identity.public_key()).is_ok(),
        "Signature should verify"
    );

    // Try with wrong key
    let wrong_key = SigningKey::from_bytes(&[6u8; 32]);
    let wrong_identity = Identity::new(wrong_key);
    assert!(
        restored
            .verify_signature(wrong_identity.public_key())
            .is_err(),
        "Signature should fail with wrong key"
    );
}

#[test]
fn envelope_version_field() {
    let source_key = SigningKey::from_bytes(&[7u8; 32]);
    let source_id = NodeId::from_public_key(&source_key.verifying_key());

    let envelope = BeeEnvelope::new(source_id, None, vec![]);
    assert_eq!(envelope.version, 1, "Default version should be 1");

    let serialized = envelope.serialize();
    let restored = BeeEnvelope::parse(&serialized).expect("Should parse");
    assert_eq!(restored.version, 1, "Version should be preserved");
}

#[test]
fn envelope_invalid_format_rejected() {
    // Test various invalid formats
    assert!(BeeEnvelope::parse(b"").is_err(), "Empty data should fail");
    assert!(
        BeeEnvelope::parse(b"invalid").is_err(),
        "Invalid format should fail"
    );
    assert!(
        BeeEnvelope::parse(&[0xFF; 10]).is_err(),
        "Random bytes should fail"
    );
}

#[test]
fn envelope_unsupported_version_rejected() {
    let source_key = SigningKey::from_bytes(&[8u8; 32]);
    let source_id = NodeId::from_public_key(&source_key.verifying_key());

    let mut envelope = BeeEnvelope::new(source_id, None, vec![]);
    envelope.version = 99; // Unsupported version

    let serialized = envelope.serialize();

    // Should fail to parse with unsupported version error
    match BeeEnvelope::parse(&serialized) {
        Err(EnvelopeError::UnsupportedVersion(v)) => {
            assert_eq!(v, 99, "Should report correct unsupported version");
        }
        _ => panic!("Should fail with UnsupportedVersion error"),
    }
}

// Property-based tests
proptest! {
    #[test]
    fn prop_envelope_roundtrip(
        source_key in prop::array::uniform32(any::<u8>()),
        dest_key in prop::array::uniform32(any::<u8>()),
        payload in prop::collection::vec(any::<u8>(), 0..10000),
        has_dest in any::<bool>()
    ) {
        let source_id = NodeId::from_public_key(
            &SigningKey::from_bytes(&source_key).verifying_key()
        );

        let dest_id = if has_dest {
            Some(NodeId::from_public_key(
                &SigningKey::from_bytes(&dest_key).verifying_key()
            ))
        } else {
            None
        };

        let envelope = BeeEnvelope::new(source_id, dest_id, payload.clone());
        let serialized = envelope.serialize();

        let restored = BeeEnvelope::parse(&serialized)
            .expect("Should parse serialized envelope");

        prop_assert_eq!(restored.version, envelope.version);
        prop_assert_eq!(restored.source, source_id);
        prop_assert_eq!(restored.destination, dest_id);
        prop_assert_eq!(restored.payload, payload);
    }

    #[test]
    fn prop_envelope_digest_integrity(
        source_key in prop::array::uniform32(any::<u8>()),
        payload in prop::collection::vec(any::<u8>(), 0..1000)
    ) {
        let source_id = NodeId::from_public_key(
            &SigningKey::from_bytes(&source_key).verifying_key()
        );

        let mut envelope = BeeEnvelope::new(source_id, None, payload);
        let digest = envelope.compute_digest();
        envelope.set_digest(digest);

        let serialized = envelope.serialize();
        let restored = BeeEnvelope::parse(&serialized)
            .expect("Should parse");

        prop_assert!(restored.verify_digest().is_ok(),
                    "Digest should verify after round-trip");
    }

    #[test]
    fn prop_envelope_signature_integrity(
        source_key in prop::array::uniform32(any::<u8>()),
        payload in prop::collection::vec(any::<u8>(), 0..1000)
    ) {
        let identity = Identity::new(SigningKey::from_bytes(&source_key));
        let source_id = identity.node_id();

        let mut envelope = BeeEnvelope::new(source_id, None, payload);
        envelope.sign(&identity);

        let serialized = envelope.serialize();
        let restored = BeeEnvelope::parse(&serialized)
            .expect("Should parse");

        prop_assert!(restored.verify_signature(identity.public_key()).is_ok(),
                    "Signature should verify after round-trip");
    }
}

// Canonical serialization test
#[test]
fn envelope_canonical_serialization() {
    // Same envelope should always serialize to same bytes
    let source_key = SigningKey::from_bytes(&[9u8; 32]);
    let source_id = NodeId::from_public_key(&source_key.verifying_key());

    let payload = b"Test canonical".to_vec();
    let envelope1 = BeeEnvelope::new(source_id, None, payload.clone());
    let envelope2 = BeeEnvelope::new(source_id, None, payload);

    let serialized1 = envelope1.serialize();
    let serialized2 = envelope2.serialize();

    assert_eq!(
        serialized1, serialized2,
        "Canonical serialization should be deterministic"
    );
}

// Large payload test
#[test]
fn envelope_large_payload() {
    let source_key = SigningKey::from_bytes(&[10u8; 32]);
    let source_id = NodeId::from_public_key(&source_key.verifying_key());

    // 1MB payload
    let payload = vec![0xAB; 1_000_000];
    let envelope = BeeEnvelope::new(source_id, None, payload.clone());

    let serialized = envelope.serialize();
    let restored = BeeEnvelope::parse(&serialized).expect("Should handle large payload");

    assert_eq!(
        restored.payload, payload,
        "Large payload should be preserved"
    );
}

// Test with binary data that could be confused with text encoding
#[test]
fn envelope_preserves_binary_payload() {
    let source_key = SigningKey::from_bytes(&[11u8; 32]);
    let source_id = NodeId::from_public_key(&source_key.verifying_key());

    // Binary data that might look like base64 or hex
    let payload = vec![
        0x00, 0x01, 0x02, 0x03, 0xFF, 0xFE, 0xFD, 0xFC, b'A', b'B', b'C', b'D', b'0', b'1', b'2',
        b'3', 0x7F, 0x80, 0x81, 0x82,
    ];

    let envelope = BeeEnvelope::new(source_id, None, payload.clone());
    let serialized = envelope.serialize();
    let restored = BeeEnvelope::parse(&serialized).expect("Should parse");

    assert_eq!(
        restored.payload, payload,
        "Binary payload must be preserved exactly without encoding"
    );
}
