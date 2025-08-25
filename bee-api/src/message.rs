use crate::error::ApiError;
use bee_core::identity::NodeId;
use serde::{Deserialize, Serialize};
use std::collections::VecDeque;
use std::sync::atomic::{AtomicU64, Ordering};
use std::time::{Duration, SystemTime};

static MESSAGE_ID_COUNTER: AtomicU64 = AtomicU64::new(1);

#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
pub struct MessageId(u64);

impl MessageId {
    fn new() -> Self {
        Self(MESSAGE_ID_COUNTER.fetch_add(1, Ordering::SeqCst))
    }
}

impl std::fmt::Display for MessageId {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        write!(f, "{}", self.0)
    }
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum MessageStatus {
    Accepted,
    Queued,
    Sent,
    Delivered,
    Failed,
    Expired,
}

impl MessageStatus {
    pub fn is_terminal(&self) -> bool {
        matches!(self, Self::Delivered | Self::Failed | Self::Expired)
    }
}

#[derive(Debug, Clone)]
pub struct Message {
    id: MessageId,
    #[allow(dead_code)]
    source: NodeId,
    #[allow(dead_code)]
    destination: NodeId,
    #[allow(dead_code)]
    payload: Vec<u8>,
    status: MessageStatus,
    created_at: SystemTime,
    queued_at: Option<SystemTime>,
    sent_at: Option<SystemTime>,
    delivered_at: Option<SystemTime>,
}

impl Message {
    pub fn new(source: NodeId, destination: NodeId, payload: Vec<u8>) -> Self {
        Self {
            id: MessageId::new(),
            source,
            destination,
            payload,
            status: MessageStatus::Accepted,
            created_at: SystemTime::now(),
            queued_at: None,
            sent_at: None,
            delivered_at: None,
        }
    }

    pub fn new_validated(
        source: NodeId,
        destination: NodeId,
        payload: Vec<u8>,
    ) -> Result<Self, ApiError> {
        const MAX_PAYLOAD_SIZE: usize = 64 * 1024;

        if payload.len() > MAX_PAYLOAD_SIZE {
            return Err(ApiError::PayloadTooLarge {
                max: MAX_PAYLOAD_SIZE,
                size: payload.len(),
            });
        }

        Ok(Self::new(source, destination, payload))
    }

    pub fn id(&self) -> MessageId {
        self.id
    }

    pub fn status(&self) -> MessageStatus {
        self.status
    }

    pub fn created_at(&self) -> SystemTime {
        self.created_at
    }

    pub fn queued_at(&self) -> Option<SystemTime> {
        self.queued_at
    }

    pub fn sent_at(&self) -> Option<SystemTime> {
        self.sent_at
    }

    pub fn delivered_at(&self) -> Option<SystemTime> {
        self.delivered_at
    }

    pub fn transition_to(&mut self, new_status: MessageStatus) -> Result<(), ApiError> {
        if !self.is_valid_transition(new_status) {
            return Err(ApiError::InvalidStatusTransition);
        }

        self.status = new_status;

        // Update timestamps
        match new_status {
            MessageStatus::Queued => self.queued_at = Some(SystemTime::now()),
            MessageStatus::Sent => self.sent_at = Some(SystemTime::now()),
            MessageStatus::Delivered => self.delivered_at = Some(SystemTime::now()),
            _ => {}
        }

        Ok(())
    }

    pub fn is_valid_transition(&self, new_status: MessageStatus) -> bool {
        if self.status.is_terminal() {
            return false;
        }

        match (self.status, new_status) {
            (MessageStatus::Accepted, MessageStatus::Queued) => true,
            (MessageStatus::Queued, MessageStatus::Sent) => true,
            (MessageStatus::Sent, MessageStatus::Delivered) => true,
            (MessageStatus::Sent, MessageStatus::Failed) => true,
            (MessageStatus::Sent, MessageStatus::Expired) => true,
            (MessageStatus::Queued, MessageStatus::Failed) => true, // For cancellation
            _ => false,
        }
    }

    pub fn cancel(&mut self) -> Result<(), ApiError> {
        match self.status {
            MessageStatus::Accepted | MessageStatus::Queued => {
                self.transition_to(MessageStatus::Failed)
            }
            _ => Err(ApiError::CannotCancelMessage(format!("{:?}", self.status))),
        }
    }
}

pub struct MessageQueue {
    messages: VecDeque<Message>,
    timeout: Duration,
    expired: Vec<Message>,
}

impl MessageQueue {
    pub fn new() -> Self {
        Self {
            messages: VecDeque::new(),
            timeout: Duration::from_secs(300),
            expired: Vec::new(),
        }
    }

    pub fn with_timeout(timeout: Duration) -> Self {
        Self {
            messages: VecDeque::new(),
            timeout,
            expired: Vec::new(),
        }
    }

    pub fn enqueue(&mut self, message: Message) {
        self.messages.push_back(message);
    }

    pub fn dequeue(&mut self) -> Option<Message> {
        self.messages.pop_front()
    }

    pub fn pending_count(&self) -> usize {
        self.messages.len()
    }

    pub fn expired_count(&self) -> usize {
        self.expired.len()
    }

    pub fn expire_old_messages(&mut self) {
        let now = SystemTime::now();
        let mut i = 0;

        while i < self.messages.len() {
            let age = now
                .duration_since(self.messages[i].created_at)
                .unwrap_or(Duration::ZERO);

            if age > self.timeout {
                let mut msg = self.messages.remove(i).unwrap();
                msg.status = MessageStatus::Expired;
                self.expired.push(msg);
            } else {
                i += 1;
            }
        }
    }
}

impl Default for MessageQueue {
    fn default() -> Self {
        Self::new()
    }
}
