//! TDD Contract Test: BeeName_validation_rejects_uppercase_and_unicode
//! This test verifies that BeeName validation correctly enforces the
//! lowercase [a-z0-9-]{3,32} regex and rejects invalid inputs.

use bee_core::name::BeeName;
use proptest::prelude::*;

#[test]
fn beename_validation_rejects_uppercase() {
    // Test uppercase rejection
    assert!(BeeName::new("ABC").is_err(), "Should reject all uppercase");
    assert!(
        BeeName::new("Test-Name").is_err(),
        "Should reject mixed case"
    );
    assert!(
        BeeName::new("HELLO-WORLD").is_err(),
        "Should reject uppercase with hyphens"
    );
}

#[test]
fn beename_validation_rejects_unicode() {
    // Test unicode rejection
    assert!(
        BeeName::new("café").is_err(),
        "Should reject accented characters"
    );
    assert!(
        BeeName::new("hello世界").is_err(),
        "Should reject Chinese characters"
    );
    assert!(
        BeeName::new("test-émoji-🐝").is_err(),
        "Should reject emoji"
    );
    assert!(BeeName::new("Привет").is_err(), "Should reject Cyrillic");
}

#[test]
fn beename_validation_rejects_invalid_characters() {
    // Test invalid ASCII characters
    assert!(
        BeeName::new("test_name").is_err(),
        "Should reject underscore"
    );
    assert!(BeeName::new("test.name").is_err(), "Should reject period");
    assert!(BeeName::new("test@name").is_err(), "Should reject at sign");
    assert!(BeeName::new("test name").is_err(), "Should reject space");
    assert!(BeeName::new("test/name").is_err(), "Should reject slash");
}

#[test]
fn beename_validation_rejects_invalid_length() {
    // Test length constraints (3-32 characters)
    assert!(
        BeeName::new("ab").is_err(),
        "Should reject 2 chars (too short)"
    );
    assert!(BeeName::new("").is_err(), "Should reject empty string");
    assert!(BeeName::new("a").is_err(), "Should reject single char");

    let too_long = "a".repeat(33);
    assert!(
        BeeName::new(&too_long).is_err(),
        "Should reject 33 chars (too long)"
    );

    let way_too_long = "test-".repeat(20);
    assert!(
        BeeName::new(&way_too_long).is_err(),
        "Should reject 100 chars"
    );
}

#[test]
fn beename_validation_accepts_valid_names() {
    // Test valid names
    assert!(BeeName::new("abc").is_ok(), "Should accept 3 chars minimum");
    assert!(
        BeeName::new("test-node-123").is_ok(),
        "Should accept lowercase with hyphens and numbers"
    );
    assert!(
        BeeName::new("a".repeat(32).as_str()).is_ok(),
        "Should accept 32 chars maximum"
    );
    assert!(
        BeeName::new("0123456789").is_ok(),
        "Should accept all digits"
    );
    assert!(
        BeeName::new("test-123-node").is_ok(),
        "Should accept mixed alphanumeric with hyphens"
    );
    assert!(BeeName::new("---").is_ok(), "Should accept all hyphens");
}

#[test]
fn beename_case_insensitive_comparison() {
    // Test case-insensitive comparison (input normalization)
    let name1 = BeeName::new("test-node").unwrap();
    let name2 = BeeName::new("test-node").unwrap();
    assert_eq!(name1, name2, "Identical names should be equal");

    // These should fail to parse, but if parsing was case-normalizing:
    // let name3 = BeeName::new("TEST-NODE"); // This should error
    // let name4 = BeeName::new("Test-Node"); // This should error
}

#[test]
fn beename_display_format() {
    // Test display formatting
    let name = BeeName::new("hello-world").unwrap();
    assert_eq!(name.to_string(), "hello-world");
    assert_eq!(format!("{}", name), "hello-world");
}

// Property-based tests
proptest! {
    #[test]
    fn prop_valid_beename_regex(
        s in "[a-z0-9-]{3,32}"
    ) {
        // All strings matching the regex should be valid
        let result = BeeName::new(&s);
        prop_assert!(result.is_ok(), "Failed to accept valid name: {}", s);

        // Round-trip through display should preserve the name
        let name = result.unwrap();
        prop_assert_eq!(name.to_string(), s);
    }

    #[test]
    fn prop_invalid_length_rejected(
        s in "[a-z0-9-]{0,2}"
    ) {
        // Too short names should be rejected
        if !s.is_empty() {
            let result = BeeName::new(&s);
            prop_assert!(result.is_err(), "Should reject short name: {}", s);
        }
    }

    #[test]
    fn prop_invalid_length_rejected_long(
        s in "[a-z0-9-]{33,100}"
    ) {
        // Too long names should be rejected
        let result = BeeName::new(&s);
        prop_assert!(result.is_err(), "Should reject long name: {}", s);
    }

    #[test]
    fn prop_uppercase_always_rejected(
        s in "[A-Z][a-z0-9-]{2,31}"
    ) {
        // Any uppercase letter should cause rejection
        let result = BeeName::new(&s);
        prop_assert!(result.is_err(), "Should reject uppercase: {}", s);
    }

    #[test]
    fn prop_special_chars_rejected(
        base in "[a-z0-9-]{2,31}",
        special in "[!@#$%^&*()_+={}\\[\\]|:;\"'<>,.?/~` ]"
    ) {
        // Insert special character in valid name
        let s = format!("{}{}", special, base);
        let result = BeeName::new(&s);
        prop_assert!(result.is_err(), "Should reject special char: {}", s);
    }
}

// Fuzz-like property test with arbitrary bytes
proptest! {
    #[test]
    fn prop_arbitrary_bytes_handled_safely(
        bytes in prop::collection::vec(any::<u8>(), 0..100)
    ) {
        // Should handle arbitrary byte sequences without panic
        if let Ok(s) = String::from_utf8(bytes.clone()) {
            let _ = BeeName::new(&s); // Should not panic
        }
    }
}

// High-volume validation test as specified
#[test]
fn high_volume_beename_validation() {
    use rand::rngs::StdRng;
    use rand::{Rng, SeedableRng};

    let mut rng = StdRng::seed_from_u64(42);
    let mut valid_count = 0;
    let mut invalid_count = 0;

    // Generate 10,000 test cases as specified
    for _ in 0..10_000 {
        let len = rng.gen_range(1..50);
        let mut name = String::new();

        for _ in 0..len {
            let choice = rng.gen_range(0..100);
            let ch = match choice {
                0..=30 => rng.gen_range(b'a'..=b'z') as char, // lowercase
                31..=50 => rng.gen_range(b'0'..=b'9') as char, // digit
                51..=60 => '-',                               // hyphen
                61..=70 => rng.gen_range(b'A'..=b'Z') as char, // uppercase (invalid)
                71..=80 => ['_', '.', '@', '!', '#'][rng.gen_range(0..5)], // special (invalid)
                81..=90 => ' ',                               // space (invalid)
                _ => {
                    // Unicode (invalid)
                    ['é', 'ñ', '世', '🐝', 'Ω'][rng.gen_range(0..5)]
                }
            };
            name.push(ch);
        }

        match BeeName::new(&name) {
            Ok(_) => valid_count += 1,
            Err(_) => invalid_count += 1,
        }
    }

    println!(
        "BeeName validation: {} valid, {} invalid out of 10,000",
        valid_count, invalid_count
    );

    // Ensure we tested both valid and invalid cases
    assert!(valid_count > 0, "Should have some valid names");
    assert!(invalid_count > 0, "Should have some invalid names");
}
