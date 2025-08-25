//! Fuzz tests for BeeEnvelope parsing
//! Run with: cargo test --test fuzz_envelope

use arbitrary::{Arbitrary, Unstructured};
use bee_core::envelope::BeeEnvelope;
use proptest::prelude::*;

#[test]
fn fuzz_bee_envelope_never_panics() {
    // Run 100k test cases with random data
    let mut panic_count = 0;
    let mut parse_success = 0;
    let mut parse_failure = 0;

    for seed in 0..100_000u64 {
        // Generate pseudo-random data using the seed
        let data = generate_fuzz_data(seed);

        // Try to parse - should never panic
        let result = std::panic::catch_unwind(|| BeeEnvelope::parse(&data));

        match result {
            Ok(parse_result) => match parse_result {
                Ok(_) => parse_success += 1,
                Err(_) => parse_failure += 1,
            },
            Err(_) => {
                panic_count += 1;
                eprintln!("PANIC on seed {}: data = {:?}", seed, data);
            }
        }
    }

    println!("Fuzz test results:");
    println!("  Total cases: 100,000");
    println!("  Parse success: {}", parse_success);
    println!("  Parse failure: {}", parse_failure);
    println!("  Panics: {}", panic_count);

    assert_eq!(panic_count, 0, "BeeEnvelope::parse should never panic!");
}

fn generate_fuzz_data(seed: u64) -> Vec<u8> {
    use std::collections::hash_map::RandomState;
    use std::hash::BuildHasher;

    let hash = RandomState::new().hash_one(seed);

    // Generate various data patterns
    match (hash % 10) as u8 {
        0 => vec![],                                  // Empty
        1 => vec![0xFF; 10000],                       // Large repeated
        2 => vec![hash as u8; (hash % 256) as usize], // Variable size
        3 => {
            // Random bytes
            let size = (hash % 1000) as usize;
            let mut data = Vec::with_capacity(size);
            let mut h = hash;
            for _ in 0..size {
                data.push(h as u8);
                h = h.wrapping_mul(1664525).wrapping_add(1013904223); // LCG
            }
            data
        }
        4 => vec![0x00],                           // Single null
        5 => vec![0xFF],                           // Single max
        6 => b"valid_looking_data_12345".to_vec(), // ASCII-like
        7 => {
            // Structured-looking data
            let mut data = vec![0x01]; // Version
            data.extend_from_slice(&[0u8; 32]); // NodeId-like
            data.extend_from_slice(&[0u8; 32]); // Another NodeId
            data.extend_from_slice(b"payload");
            data
        }
        8 => vec![hash as u8, (hash >> 8) as u8, (hash >> 16) as u8],
        _ => vec![0x42; 42],
    }
}

// Property-based fuzzing with proptest
proptest! {
    #[test]
    fn envelope_parse_never_panics_proptest(data: Vec<u8>) {
        // Should never panic regardless of input
        let _ = BeeEnvelope::parse(&data);
    }

    #[test]
    fn envelope_parse_handles_arbitrary_data(
        data in prop::collection::vec(any::<u8>(), 0..10000)
    ) {
        // Parse should handle any byte sequence without panicking
        let result = std::panic::catch_unwind(|| {
            BeeEnvelope::parse(&data)
        });

        // Assert no panic occurred
        prop_assert!(result.is_ok(), "Parse panicked on input: {:?}", data);
    }
}

// Test with Arbitrary trait for more complex fuzzing
#[derive(Debug, Arbitrary)]
struct FuzzInput {
    version: u8,
    source_bytes: [u8; 32],
    dest_bytes: Option<[u8; 32]>,
    payload_size: u16,
    payload_pattern: u8,
}

#[test]
fn fuzz_with_structured_input() {
    for seed in 0..10_000u32 {
        let data = vec![seed as u8; 1024];
        let mut u = Unstructured::new(&data);

        if let Ok(input) = FuzzInput::arbitrary(&mut u) {
            // Convert structured input to bytes
            let mut bytes = vec![input.version];
            bytes.extend_from_slice(&input.source_bytes);

            if let Some(dest) = input.dest_bytes {
                bytes.push(1); // Has destination flag
                bytes.extend_from_slice(&dest);
            } else {
                bytes.push(0); // No destination flag
            }

            // Add payload
            let payload = vec![input.payload_pattern; input.payload_size as usize];
            bytes.extend_from_slice(&(input.payload_size as u32).to_be_bytes());
            bytes.extend_from_slice(&payload);

            // Should not panic
            let _ = BeeEnvelope::parse(&bytes);
        }
    }
}
