mod common;

use bee_api::{Message, MessageStatus, MessageQueue, ApiError};
use std::time::Duration;

#[test]
fn test_message_status_starts_accepted() {
    let message = Message::new(
        common::test_node_id(1),
        common::test_node_id(1),
        vec![1, 2, 3],
    );
    assert_eq!(message.status(), MessageStatus::Accepted);
}

#[test]
fn test_message_transitions_accepted_to_queued() {
    let mut message = Message::new(
        common::test_node_id(1),
        common::test_node_id(1),
        vec![1, 2, 3],
    );
    
    assert!(message.transition_to(MessageStatus::Queued).is_ok());
    assert_eq!(message.status(), MessageStatus::Queued);
    
    // Cannot go back to Accepted
    assert!(message.transition_to(MessageStatus::Accepted).is_err());
}

#[test]
fn test_message_transitions_queued_to_sent() {
    let mut message = Message::new(
        common::test_node_id(1),
        common::test_node_id(1),
        vec![1, 2, 3],
    );
    
    message.transition_to(MessageStatus::Queued).unwrap();
    assert!(message.transition_to(MessageStatus::Sent).is_ok());
    assert_eq!(message.status(), MessageStatus::Sent);
}

#[test]
fn test_message_transitions_sent_to_delivered() {
    let mut message = Message::new(
        common::test_node_id(1),
        common::test_node_id(1),
        vec![1, 2, 3],
    );
    
    message.transition_to(MessageStatus::Queued).unwrap();
    message.transition_to(MessageStatus::Sent).unwrap();
    assert!(message.transition_to(MessageStatus::Delivered).is_ok());
    assert_eq!(message.status(), MessageStatus::Delivered);
}

#[test]
fn test_message_transitions_sent_to_failed() {
    let mut message = Message::new(
        common::test_node_id(1),
        common::test_node_id(1),
        vec![1, 2, 3],
    );
    
    message.transition_to(MessageStatus::Queued).unwrap();
    message.transition_to(MessageStatus::Sent).unwrap();
    assert!(message.transition_to(MessageStatus::Failed).is_ok());
    assert_eq!(message.status(), MessageStatus::Failed);
}

#[test]
fn test_message_transitions_sent_to_expired() {
    let mut message = Message::new(
        common::test_node_id(1),
        common::test_node_id(1),
        vec![1, 2, 3],
    );
    
    message.transition_to(MessageStatus::Queued).unwrap();
    message.transition_to(MessageStatus::Sent).unwrap();
    assert!(message.transition_to(MessageStatus::Expired).is_ok());
    assert_eq!(message.status(), MessageStatus::Expired);
}

#[test]
fn test_illegal_transition_accepted_to_delivered() {
    let mut message = Message::new(
        common::test_node_id(1),
        common::test_node_id(1),
        vec![1, 2, 3],
    );
    
    // Cannot jump directly from Accepted to Delivered
    assert!(message.transition_to(MessageStatus::Delivered).is_err());
}

#[test]
fn test_illegal_transition_queued_to_delivered() {
    let mut message = Message::new(
        common::test_node_id(1),
        common::test_node_id(1),
        vec![1, 2, 3],
    );
    
    message.transition_to(MessageStatus::Queued).unwrap();
    // Cannot jump directly from Queued to Delivered
    assert!(message.transition_to(MessageStatus::Delivered).is_err());
}

#[test]
fn test_terminal_states_cannot_transition() {
    let test_terminal = |terminal_status: MessageStatus| {
        let mut message = Message::new(
            common::test_node_id(1),
            common::test_node_id(1),
            vec![1, 2, 3],
        );
        
        // Move to terminal state
        message.transition_to(MessageStatus::Queued).unwrap();
        message.transition_to(MessageStatus::Sent).unwrap();
        message.transition_to(terminal_status).unwrap();
        
        // Cannot transition from terminal state
        for status in [
            MessageStatus::Accepted,
            MessageStatus::Queued,
            MessageStatus::Sent,
            MessageStatus::Delivered,
            MessageStatus::Failed,
            MessageStatus::Expired,
        ] {
            assert!(message.transition_to(status).is_err());
        }
    };
    
    test_terminal(MessageStatus::Delivered);
    test_terminal(MessageStatus::Failed);
    test_terminal(MessageStatus::Expired);
}

#[test]
fn test_message_id_uniqueness() {
    let msg1 = Message::new(
        common::test_node_id(1),
        common::test_node_id(1),
        vec![1, 2, 3],
    );
    
    let msg2 = Message::new(
        common::test_node_id(1),
        common::test_node_id(1),
        vec![1, 2, 3],
    );
    
    assert_ne!(msg1.id(), msg2.id());
}

#[test]
fn test_message_timestamp_set_on_creation() {
    let before = std::time::SystemTime::now();
    let message = Message::new(
        common::test_node_id(1),
        common::test_node_id(1),
        vec![1, 2, 3],
    );
    let after = std::time::SystemTime::now();
    
    assert!(message.created_at() >= before);
    assert!(message.created_at() <= after);
}

#[test]
fn test_message_queue_fifo_ordering() {
    
    let mut queue = MessageQueue::new();
    
    let msg1 = Message::new(
        common::test_node_id(1),
        common::test_node_id(1),
        vec![1],
    );
    let msg2 = Message::new(
        common::test_node_id(1),
        common::test_node_id(1),
        vec![2],
    );
    let msg3 = Message::new(
        common::test_node_id(1),
        common::test_node_id(1),
        vec![3],
    );
    
    let id1 = msg1.id();
    let id2 = msg2.id();
    let id3 = msg3.id();
    
    queue.enqueue(msg1);
    queue.enqueue(msg2);
    queue.enqueue(msg3);
    
    assert_eq!(queue.dequeue().unwrap().id(), id1);
    assert_eq!(queue.dequeue().unwrap().id(), id2);
    assert_eq!(queue.dequeue().unwrap().id(), id3);
    assert!(queue.dequeue().is_none());
}

#[test]
fn test_message_expiration_after_timeout() {
    
    let mut queue = MessageQueue::with_timeout(Duration::from_millis(100));
    
    let mut message = Message::new(
        common::test_node_id(1),
        common::test_node_id(1),
        vec![1, 2, 3],
    );
    message.transition_to(MessageStatus::Queued).unwrap();
    
    queue.enqueue(message);
    
    // Before timeout
    assert_eq!(queue.pending_count(), 1);
    
    // Simulate time passing
    std::thread::sleep(Duration::from_millis(150));
    
    // After timeout, message should be expired
    queue.expire_old_messages();
    assert_eq!(queue.pending_count(), 0);
    assert_eq!(queue.expired_count(), 1);
}

#[test]
fn test_message_cancel_while_queued() {
    let mut message = Message::new(
        common::test_node_id(1),
        common::test_node_id(1),
        vec![1, 2, 3],
    );
    
    message.transition_to(MessageStatus::Queued).unwrap();
    assert!(message.cancel().is_ok());
    assert_eq!(message.status(), MessageStatus::Failed);
}

#[test]
fn test_message_cannot_cancel_after_sent() {
    let mut message = Message::new(
        common::test_node_id(1),
        common::test_node_id(1),
        vec![1, 2, 3],
    );
    
    message.transition_to(MessageStatus::Queued).unwrap();
    message.transition_to(MessageStatus::Sent).unwrap();
    assert!(message.cancel().is_err());
}

#[test]
fn test_message_payload_size_limit() {
    // Maximum payload size should be enforced
    const MAX_PAYLOAD_SIZE: usize = 64 * 1024; // 64KB
    
    let large_payload = vec![0u8; MAX_PAYLOAD_SIZE + 1];
    let result = Message::new_validated(
        common::test_node_id(1),
        common::test_node_id(1),
        large_payload,
    );
    
    assert!(result.is_err());
    assert!(matches!(result.unwrap_err(), ApiError::PayloadTooLarge { .. }));
}

#[test]
fn test_message_metadata_tracking() {
    let mut message = Message::new(
        common::test_node_id(1),
        common::test_node_id(1),
        vec![1, 2, 3],
    );
    
    // Track status transition times
    let t1 = std::time::SystemTime::now();
    message.transition_to(MessageStatus::Queued).unwrap();
    
    std::thread::sleep(Duration::from_millis(10));
    
    let t2 = std::time::SystemTime::now();
    message.transition_to(MessageStatus::Sent).unwrap();
    
    std::thread::sleep(Duration::from_millis(10));
    
    let t3 = std::time::SystemTime::now();
    message.transition_to(MessageStatus::Delivered).unwrap();
    
    // Verify timestamps are recorded
    assert!(message.queued_at().unwrap() >= t1);
    assert!(message.sent_at().unwrap() >= t2);
    assert!(message.delivered_at().unwrap() >= t3);
}

#[cfg(test)]
mod property_tests {
    use super::*;
    use proptest::prelude::*;
    
    proptest! {
        #[test]
        fn test_status_transitions_are_linear(
            transitions in prop::collection::vec(0..6u8, 0..20)
        ) {
            let mut message = Message::new(
                common::test_node_id(1),
                common::test_node_id(1),
                vec![1, 2, 3],
            );
            
            let statuses = [
                MessageStatus::Accepted,
                MessageStatus::Queued,
                MessageStatus::Sent,
                MessageStatus::Delivered,
                MessageStatus::Failed,
                MessageStatus::Expired,
            ];
            
            let mut reached_terminal = false;
            
            for t in transitions {
                if reached_terminal {
                    // No transitions allowed after terminal state
                    let status = statuses[t as usize % 6];
                    assert!(message.transition_to(status).is_err());
                } else {
                    let status = statuses[t as usize % 6];
                    
                    // Check if transition is valid according to state machine
                    if message.is_valid_transition(status) {
                        let result = message.transition_to(status);
                        assert!(result.is_ok());
                        if status.is_terminal() {
                            reached_terminal = true;
                        }
                    } else {
                        let result = message.transition_to(status);
                        assert!(result.is_err());
                    }
                }
            }
        }
    }
}