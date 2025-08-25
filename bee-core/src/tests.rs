use crate::clock::{Clock, MockClock};
use crate::envelope::{BeeEnvelope, EnvelopeError};
use crate::identity::{Identity, NodeId};
use proptest::prelude::*;
use std::time::Duration;

#[test]
fn core_clock_advances_only_when_ticked() {
    let mut clock = MockClock::new();
    let initial_time = clock.now();

    // Time should not advance without tick
    assert_eq!(clock.now(), initial_time);
    assert_eq!(clock.now(), initial_time);

    // Time should advance when ticked
    clock.tick();
    let time_after_tick = clock.now();
    assert!(time_after_tick > initial_time);

    // Time should remain constant between ticks
    assert_eq!(clock.now(), time_after_tick);
    assert_eq!(clock.now(), time_after_tick);

    // Each tick should advance by the same amount
    clock.tick();
    let time_after_second_tick = clock.now();
    let first_delta = time_after_tick - initial_time;
    let second_delta = time_after_second_tick - time_after_tick;
    assert_eq!(first_delta, second_delta);

    // Advance should work as expected
    let advance_amount = Duration::from_secs(100);
    clock.advance(advance_amount);
    assert_eq!(clock.now(), time_after_second_tick + advance_amount);
}

// Property-based test for clock monotonicity
proptest! {
    #[test]
    fn clock_time_is_monotonic(
        tick_count in 0..1000u32,
        advance_amounts in prop::collection::vec(0..10000u64, 0..100)
    ) {
        let mut clock = MockClock::new();
        let mut last_time = clock.now();

        for _ in 0..tick_count {
            clock.tick();
            let current_time = clock.now();
            prop_assert!(current_time >= last_time);
            last_time = current_time;
        }

        for amount in advance_amounts {
            clock.advance(Duration::from_millis(amount));
            let current_time = clock.now();
            prop_assert!(current_time >= last_time);
            last_time = current_time;
        }
    }
}

#[test]
fn identity_node_id_deterministic() {
    use ed25519_dalek::SigningKey;

    // Same key should produce same NodeId
    let key_bytes = [42u8; 32];
    let signing_key1 = SigningKey::from_bytes(&key_bytes);
    let signing_key2 = SigningKey::from_bytes(&key_bytes);

    let identity1 = Identity::new(signing_key1);
    let identity2 = Identity::new(signing_key2);

    assert_eq!(identity1.node_id(), identity2.node_id());
}

#[test]
fn envelope_parse_rejects_invalid_data() {
    // Test that malformed data is rejected
    let invalid_data = vec![0xFF, 0xFF, 0xFF];
    let result = BeeEnvelope::parse(&invalid_data);
    assert!(matches!(result, Err(EnvelopeError::InvalidFormat)));

    // Test empty data
    let result = BeeEnvelope::parse(&[]);
    assert!(matches!(result, Err(EnvelopeError::InvalidFormat)));
}
