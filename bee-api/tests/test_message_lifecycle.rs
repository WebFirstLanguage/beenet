mod common;

use bee_api::{ApiError, Message, MessageQueue, MessageStatus};
use bee_core::clock::{Clock, MockClock};
use std::time::Duration;

#[test]
fn test_message_status_starts_accepted() {
    let clock = MockClock::new();
    let message = Message::new(
        &clock,
        common::test_node_id(1),
        common::test_node_id(1),
        b"test payload".to_vec(),
    );
    assert_eq!(message.status(), MessageStatus::Accepted);
}

#[test]
fn test_basic_status_transitions() {
    let clock = MockClock::new();
    let mut message = Message::new(
        &clock,
        common::test_node_id(1),
        common::test_node_id(2),
        b"test payload".to_vec(),
    );

    assert!(message.transition_to(&clock, MessageStatus::Queued).is_ok());
    assert_eq!(message.status(), MessageStatus::Queued);

    // Should not allow backward transitions
    assert!(message
        .transition_to(&clock, MessageStatus::Accepted)
        .is_err());
}

#[test]
fn test_queued_to_sent_transition() {
    let clock = MockClock::new();
    let mut message = Message::new(
        &clock,
        common::test_node_id(1),
        common::test_node_id(2),
        b"test payload".to_vec(),
    );

    message
        .transition_to(&clock, MessageStatus::Queued)
        .unwrap();
    assert!(message.transition_to(&clock, MessageStatus::Sent).is_ok());
    assert_eq!(message.status(), MessageStatus::Sent);
}

#[test]
fn test_sent_to_delivered_transition() {
    let clock = MockClock::new();
    let mut message = Message::new(
        &clock,
        common::test_node_id(1),
        common::test_node_id(2),
        b"test payload".to_vec(),
    );

    message
        .transition_to(&clock, MessageStatus::Queued)
        .unwrap();
    message.transition_to(&clock, MessageStatus::Sent).unwrap();
    assert!(message
        .transition_to(&clock, MessageStatus::Delivered)
        .is_ok());
    assert_eq!(message.status(), MessageStatus::Delivered);
}

#[test]
fn test_sent_to_failed_transition() {
    let clock = MockClock::new();
    let mut message = Message::new(
        &clock,
        common::test_node_id(1),
        common::test_node_id(2),
        b"test payload".to_vec(),
    );

    message
        .transition_to(&clock, MessageStatus::Queued)
        .unwrap();
    message.transition_to(&clock, MessageStatus::Sent).unwrap();
    assert!(message.transition_to(&clock, MessageStatus::Failed).is_ok());
    assert_eq!(message.status(), MessageStatus::Failed);
}

#[test]
fn test_sent_to_expired_transition() {
    let clock = MockClock::new();
    let mut message = Message::new(
        &clock,
        common::test_node_id(1),
        common::test_node_id(2),
        b"test payload".to_vec(),
    );

    message
        .transition_to(&clock, MessageStatus::Queued)
        .unwrap();
    message.transition_to(&clock, MessageStatus::Sent).unwrap();
    assert!(message
        .transition_to(&clock, MessageStatus::Expired)
        .is_ok());
    assert_eq!(message.status(), MessageStatus::Expired);
}

#[test]
fn test_invalid_direct_to_terminal_states() {
    let clock = MockClock::new();
    let mut message = Message::new(
        &clock,
        common::test_node_id(1),
        common::test_node_id(2),
        b"test payload".to_vec(),
    );

    // Cannot go directly to terminal states
    assert!(message
        .transition_to(&clock, MessageStatus::Delivered)
        .is_err());
}

#[test]
fn test_invalid_queued_to_delivered_skip() {
    let clock = MockClock::new();
    let mut message = Message::new(
        &clock,
        common::test_node_id(1),
        common::test_node_id(2),
        b"test payload".to_vec(),
    );

    message
        .transition_to(&clock, MessageStatus::Queued)
        .unwrap();

    // Cannot skip Sent state
    assert!(message
        .transition_to(&clock, MessageStatus::Delivered)
        .is_err());
}

#[test]
fn test_terminal_states_reject_all_transitions() {
    let clock = MockClock::new();
    for &terminal_status in &[
        MessageStatus::Delivered,
        MessageStatus::Failed,
        MessageStatus::Expired,
    ] {
        let mut message = Message::new(
            &clock,
            common::test_node_id(1),
            common::test_node_id(2),
            b"test payload".to_vec(),
        );

        message
            .transition_to(&clock, MessageStatus::Queued)
            .unwrap();
        message.transition_to(&clock, MessageStatus::Sent).unwrap();
        message.transition_to(&clock, terminal_status).unwrap();

        // Try all possible transitions from terminal state - all should fail
        for &status in &[
            MessageStatus::Accepted,
            MessageStatus::Queued,
            MessageStatus::Sent,
            MessageStatus::Delivered,
            MessageStatus::Failed,
            MessageStatus::Expired,
        ] {
            assert!(message.transition_to(&clock, status).is_err());
        }
    }
}

#[test]
fn test_message_ids_are_unique() {
    let clock = MockClock::new();
    let msg1 = Message::new(
        &clock,
        common::test_node_id(1),
        common::test_node_id(2),
        b"test1".to_vec(),
    );

    let msg2 = Message::new(
        &clock,
        common::test_node_id(1),
        common::test_node_id(2),
        b"test2".to_vec(),
    );

    assert_ne!(msg1.id(), msg2.id());
}

#[test]
fn test_timestamp_tracking_creation() {
    let clock = MockClock::new();
    let before = clock.now();

    let message = Message::new(
        &clock,
        common::test_node_id(1),
        common::test_node_id(2),
        b"test".to_vec(),
    );

    // Created at time should be exactly when we created it
    assert_eq!(message.created_at(), before);

    // Other timestamps should be None initially
    assert!(message.queued_at().is_none());
    assert!(message.sent_at().is_none());
    assert!(message.delivered_at().is_none());
}

#[test]
fn test_queue_maintains_fifo_order() {
    let clock = MockClock::new();
    let mut queue = MessageQueue::new();

    let msg1 = Message::new(
        &clock,
        common::test_node_id(1),
        common::test_node_id(2),
        b"first".to_vec(),
    );
    let msg2 = Message::new(
        &clock,
        common::test_node_id(1),
        common::test_node_id(2),
        b"second".to_vec(),
    );
    let msg3 = Message::new(
        &clock,
        common::test_node_id(1),
        common::test_node_id(2),
        b"third".to_vec(),
    );

    let id1 = msg1.id();
    let id2 = msg2.id();
    let id3 = msg3.id();

    queue.enqueue(msg1);
    queue.enqueue(msg2);
    queue.enqueue(msg3);

    assert_eq!(queue.pending_count(), 3);

    // Should dequeue in FIFO order
    let dequeued1 = queue.dequeue().unwrap();
    let dequeued2 = queue.dequeue().unwrap();
    let dequeued3 = queue.dequeue().unwrap();

    assert_eq!(dequeued1.id(), id1);
    assert_eq!(dequeued2.id(), id2);
    assert_eq!(dequeued3.id(), id3);

    assert_eq!(queue.pending_count(), 0);
    assert!(queue.dequeue().is_none());
}

#[test]
fn test_queue_message_expiration() {
    let mut clock = MockClock::new();
    let mut queue = MessageQueue::with_timeout(Duration::from_secs(5));

    let mut message = Message::new(
        &clock,
        common::test_node_id(1),
        common::test_node_id(2),
        b"test".to_vec(),
    );

    message
        .transition_to(&clock, MessageStatus::Queued)
        .unwrap();
    queue.enqueue(message);

    assert_eq!(queue.pending_count(), 1);
    assert_eq!(queue.expired_count(), 0);

    // Advance time beyond timeout and expire
    clock.advance(Duration::from_secs(10));
    queue.expire_old_messages(&clock);

    assert_eq!(queue.pending_count(), 0);
    assert_eq!(queue.expired_count(), 1);
}

#[test]
fn test_cancel_message_when_accepted() {
    let clock = MockClock::new();
    let mut message = Message::new(
        &clock,
        common::test_node_id(1),
        common::test_node_id(2),
        b"test".to_vec(),
    );

    assert!(message.cancel(&clock).is_ok());
    assert_eq!(message.status(), MessageStatus::Failed);
}

#[test]
fn test_cancel_message_when_queued() {
    let clock = MockClock::new();
    let mut message = Message::new(
        &clock,
        common::test_node_id(1),
        common::test_node_id(2),
        b"test".to_vec(),
    );

    message
        .transition_to(&clock, MessageStatus::Queued)
        .unwrap();
    assert!(message.cancel(&clock).is_ok());
    assert_eq!(message.status(), MessageStatus::Failed);
}

#[test]
fn test_cannot_cancel_sent_message() {
    let clock = MockClock::new();
    let mut message = Message::new(
        &clock,
        common::test_node_id(1),
        common::test_node_id(2),
        b"test".to_vec(),
    );

    message
        .transition_to(&clock, MessageStatus::Queued)
        .unwrap();
    message.transition_to(&clock, MessageStatus::Sent).unwrap();

    assert!(message.cancel(&clock).is_err());
    assert_eq!(message.status(), MessageStatus::Sent);
}

#[test]
fn test_payload_size_limit_enforcement() {
    let clock = MockClock::new();
    let large_payload = vec![0u8; 65 * 1024]; // 65KB - exceeds 64KB limit

    let result = Message::new_validated(
        &clock,
        common::test_node_id(1),
        common::test_node_id(2),
        large_payload,
    );

    assert!(result.is_err());
    assert!(matches!(
        result.unwrap_err(),
        ApiError::PayloadTooLarge { .. }
    ));
}

#[test]
fn test_metadata_timestamps_during_transitions() {
    let mut clock = MockClock::new();
    let mut message = Message::new(
        &clock,
        common::test_node_id(1),
        common::test_node_id(2),
        b"test".to_vec(),
    );

    let t1 = clock.now();

    // Advance time and transition to Queued
    clock.advance(Duration::from_millis(10));
    message
        .transition_to(&clock, MessageStatus::Queued)
        .unwrap();
    let t2 = clock.now();

    // Advance time and transition to Sent
    clock.advance(Duration::from_millis(10));
    message.transition_to(&clock, MessageStatus::Sent).unwrap();
    let t3 = clock.now();

    // Transition to Delivered (no time advance)
    message
        .transition_to(&clock, MessageStatus::Delivered)
        .unwrap();

    // Verify timestamps
    assert_eq!(message.created_at(), t1);
    assert_eq!(message.queued_at(), Some(t2));
    assert_eq!(message.sent_at(), Some(t3));
    assert_eq!(message.delivered_at(), Some(t3));
}

#[cfg(test)]
mod property_tests {
    use super::*;
    use proptest::prelude::*;

    prop_compose! {
        fn message_status_strategy()(status in 0..6u8) -> MessageStatus {
            match status {
                0 => MessageStatus::Accepted,
                1 => MessageStatus::Queued,
                2 => MessageStatus::Sent,
                3 => MessageStatus::Delivered,
                4 => MessageStatus::Failed,
                _ => MessageStatus::Expired,
            }
        }
    }

    proptest! {
        #[test]
        fn property_transition_consistency(
            statuses in prop::collection::vec(message_status_strategy(), 1..10)
        ) {
            let clock = MockClock::new();
            let mut message = Message::new(
                &clock,
                common::test_node_id(1),
                common::test_node_id(2),
                b"test".to_vec(),
            );

            for status in statuses {
                if message.status().is_terminal() {
                    // Once terminal, all transitions should fail
                    assert!(message.transition_to(&clock, status).is_err());
                } else {
                    let result = message.transition_to(&clock, status);
                    if result.is_ok() {
                        // Valid transition occurred
                        assert_eq!(message.status(), status);
                    } else {
                        // Invalid transition was rejected - status unchanged
                        // Try the transition again to make sure status didn't change
                        let old_status = message.status();
                        let result2 = message.transition_to(&clock, status);
                        assert!(result2.is_err());
                        assert_eq!(message.status(), old_status);
                    }
                }
            }
        }
    }
}
