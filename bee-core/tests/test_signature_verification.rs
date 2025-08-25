//! TDD Contract Test: Signature_verifies_and_refuses_mismatch
//! This test verifies Ed25519 signature generation and verification,
//! ensuring signatures are accepted when valid and rejected when invalid.

use bee_core::identity::Identity;
use ed25519_dalek::{Signature, SigningKey, Verifier};
use proptest::prelude::*;

#[test]
fn signature_verifies_with_correct_key() {
    let signing_key = SigningKey::from_bytes(&[1u8; 32]);
    let identity = Identity::new(signing_key.clone());

    let message = b"Hello, Beenet!";
    let signature = identity.sign(message);

    // Verify with the correct public key
    let result = identity.public_key().verify(message, &signature);
    assert!(result.is_ok(), "Valid signature should verify");
}

#[test]
fn signature_refuses_wrong_message() {
    let signing_key = SigningKey::from_bytes(&[2u8; 32]);
    let identity = Identity::new(signing_key);

    let message = b"Original message";
    let signature = identity.sign(message);

    // Try to verify with different message
    let wrong_message = b"Modified message";
    let result = identity.public_key().verify(wrong_message, &signature);
    assert!(result.is_err(), "Signature should fail for wrong message");
}

#[test]
fn signature_refuses_wrong_key() {
    let signing_key1 = SigningKey::from_bytes(&[3u8; 32]);
    let signing_key2 = SigningKey::from_bytes(&[4u8; 32]);

    let identity1 = Identity::new(signing_key1);
    let identity2 = Identity::new(signing_key2);

    let message = b"Test message";
    let signature = identity1.sign(message);

    // Try to verify with wrong public key
    let result = identity2.public_key().verify(message, &signature);
    assert!(result.is_err(), "Signature should fail with wrong key");
}

#[test]
fn signature_refuses_tampered_signature() {
    let signing_key = SigningKey::from_bytes(&[5u8; 32]);
    let identity = Identity::new(signing_key);

    let message = b"Authentic message";
    let signature = identity.sign(message);

    // Tamper with the signature
    let mut tampered_bytes = signature.to_bytes();
    tampered_bytes[0] ^= 0xFF; // Flip bits in first byte
    let tampered_signature = Signature::from_bytes(&tampered_bytes);

    let result = identity.public_key().verify(message, &tampered_signature);
    assert!(
        result.is_err(),
        "Tampered signature should fail verification"
    );
}

#[test]
fn signature_empty_message() {
    let signing_key = SigningKey::from_bytes(&[6u8; 32]);
    let identity = Identity::new(signing_key);

    let message = b"";
    let signature = identity.sign(message);

    let result = identity.public_key().verify(message, &signature);
    assert!(
        result.is_ok(),
        "Should be able to sign and verify empty message"
    );
}

#[test]
fn signature_large_message() {
    let signing_key = SigningKey::from_bytes(&[7u8; 32]);
    let identity = Identity::new(signing_key);

    let message = vec![0xAB; 100_000]; // 100KB message
    let signature = identity.sign(&message);

    let result = identity.public_key().verify(&message, &signature);
    assert!(result.is_ok(), "Should handle large messages");
}

#[test]
fn multiple_signatures_different() {
    // Ed25519 is deterministic, same message should produce same signature
    let signing_key = SigningKey::from_bytes(&[8u8; 32]);
    let identity = Identity::new(signing_key);

    let message = b"Deterministic test";
    let sig1 = identity.sign(message);
    let sig2 = identity.sign(message);

    assert_eq!(sig1, sig2, "Ed25519 signatures should be deterministic");
}

// Property-based tests
proptest! {
    #[test]
    fn prop_signature_verifies(
        key_bytes in prop::array::uniform32(any::<u8>()),
        message in prop::collection::vec(any::<u8>(), 0..1000)
    ) {
        let signing_key = SigningKey::from_bytes(&key_bytes);
        let identity = Identity::new(signing_key);

        let signature = identity.sign(&message);
        let result = identity.public_key().verify(&message, &signature);

        prop_assert!(result.is_ok(), "Valid signature must verify");
    }

    #[test]
    fn prop_signature_deterministic(
        key_bytes in prop::array::uniform32(any::<u8>()),
        message in prop::collection::vec(any::<u8>(), 0..1000)
    ) {
        let signing_key = SigningKey::from_bytes(&key_bytes);
        let identity = Identity::new(signing_key);

        let sig1 = identity.sign(&message);
        let sig2 = identity.sign(&message);

        prop_assert_eq!(sig1, sig2, "Ed25519 signatures must be deterministic");
    }

    #[test]
    fn prop_wrong_key_fails(
        key1 in prop::array::uniform32(any::<u8>()),
        key2 in prop::array::uniform32(any::<u8>()),
        message in prop::collection::vec(any::<u8>(), 0..1000)
    ) {
        prop_assume!(key1 != key2);

        let identity1 = Identity::new(SigningKey::from_bytes(&key1));
        let identity2 = Identity::new(SigningKey::from_bytes(&key2));

        let signature = identity1.sign(&message);
        let result = identity2.public_key().verify(&message, &signature);

        prop_assert!(result.is_err(), "Signature with wrong key must fail");
    }

    #[test]
    fn prop_tampered_message_fails(
        key_bytes in prop::array::uniform32(any::<u8>()),
        message in prop::collection::vec(any::<u8>(), 1..1000),
        tamper_idx in any::<usize>()
    ) {
        let signing_key = SigningKey::from_bytes(&key_bytes);
        let identity = Identity::new(signing_key);

        let signature = identity.sign(&message);

        // Tamper with the message
        let mut tampered = message.clone();
        let idx = tamper_idx % tampered.len();
        tampered[idx] ^= 0xFF;

        let result = identity.public_key().verify(&tampered, &signature);
        prop_assert!(result.is_err(), "Signature with tampered message must fail");
    }

    #[test]
    fn prop_tampered_signature_fails(
        key_bytes in prop::array::uniform32(any::<u8>()),
        message in prop::collection::vec(any::<u8>(), 0..1000),
        tamper_idx in 0usize..64
    ) {
        let signing_key = SigningKey::from_bytes(&key_bytes);
        let identity = Identity::new(signing_key);

        let signature = identity.sign(&message);

        // Tamper with the signature
        let mut sig_bytes = signature.to_bytes();
        sig_bytes[tamper_idx] ^= 0xFF;
        let tampered_sig = Signature::from_bytes(&sig_bytes);

        let result = identity.public_key().verify(&message, &tampered_sig);
        prop_assert!(result.is_err(), "Tampered signature must fail");
    }
}

// High-volume signature verification test (100k as specified)
#[test]
fn high_volume_signature_verification() {
    use rand::rngs::StdRng;
    use rand::{Rng, SeedableRng};

    let mut rng = StdRng::seed_from_u64(54321);
    let mut valid_count = 0;
    let mut invalid_count = 0;

    println!("Running 100,000 signature verification tests...");

    for i in 0..100_000 {
        if i % 10_000 == 0 {
            println!("Progress: {}/100,000", i);
        }

        // Generate random key and message
        let mut key_bytes = [0u8; 32];
        rng.fill(&mut key_bytes);
        let signing_key = SigningKey::from_bytes(&key_bytes);
        let identity = Identity::new(signing_key);

        let msg_len = rng.gen_range(0..1000);
        let mut message = vec![0u8; msg_len];
        rng.fill(&mut message[..]);

        // Sign the message
        let signature = identity.sign(&message);

        // Test valid verification
        let valid_result = identity.public_key().verify(&message, &signature);
        assert!(valid_result.is_ok(), "Valid signature should verify");
        valid_count += 1;

        // Test invalid verification (wrong message)
        if !message.is_empty() {
            let mut wrong_message = message.clone();
            wrong_message[0] ^= 0xFF;
            let invalid_result = identity.public_key().verify(&wrong_message, &signature);
            assert!(invalid_result.is_err(), "Invalid message should fail");
            invalid_count += 1;
        }
    }

    println!("Signature verification complete:");
    println!("  Valid verifications: {}", valid_count);
    println!("  Invalid rejections: {}", invalid_count);
    assert_eq!(valid_count, 100_000, "All valid signatures should verify");
}

// Test signature serialization/deserialization
#[test]
fn signature_serialization_roundtrip() {
    let signing_key = SigningKey::from_bytes(&[9u8; 32]);
    let identity = Identity::new(signing_key);

    let message = b"Test serialization";
    let signature = identity.sign(message);

    // Convert to bytes and back
    let sig_bytes = signature.to_bytes();
    let restored_sig = Signature::from_bytes(&sig_bytes);

    // Verify the restored signature
    let result = identity.public_key().verify(message, &restored_sig);
    assert!(result.is_ok(), "Restored signature should verify");
    assert_eq!(
        signature, restored_sig,
        "Signature should survive serialization"
    );
}
