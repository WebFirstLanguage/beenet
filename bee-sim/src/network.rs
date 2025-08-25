use bee_core::clock::Clock;
use bee_core::identity::NodeId;
use std::collections::{HashMap, VecDeque};
use std::time::Duration;

/// Network packet in transit
#[derive(Debug, Clone)]
pub struct Packet {
    pub source: NodeId,
    pub destination: NodeId,
    pub data: Vec<u8>,
    pub delivery_time: Duration,
}

/// Deterministic network simulator
pub struct SimNet<C: Clock> {
    clock: C,
    nodes: HashMap<NodeId, NodeState>,
    packets_in_transit: VecDeque<Packet>,
    default_latency: Duration,
    default_loss_rate: f32,
}

#[derive(Debug, Default)]
struct NodeState {
    online: bool,
    inbox: VecDeque<Vec<u8>>,
}

impl<C: Clock> SimNet<C> {
    pub fn new(clock: C) -> Self {
        Self {
            clock,
            nodes: HashMap::new(),
            packets_in_transit: VecDeque::new(),
            default_latency: Duration::from_millis(10),
            default_loss_rate: 0.0,
        }
    }

    pub fn add_node(&mut self, node_id: NodeId) {
        self.nodes.insert(
            node_id,
            NodeState {
                online: true,
                inbox: VecDeque::new(),
            },
        );
    }

    pub fn send(
        &mut self,
        source: NodeId,
        destination: NodeId,
        data: Vec<u8>,
    ) -> Result<(), NetworkError> {
        if !self.nodes.contains_key(&source) {
            return Err(NetworkError::UnknownNode(source));
        }
        if !self.nodes.contains_key(&destination) {
            return Err(NetworkError::UnknownNode(destination));
        }

        // Deterministic packet loss based on hash of packet content
        let mut hasher = std::collections::hash_map::DefaultHasher::new();
        use std::hash::{Hash, Hasher};
        data.hash(&mut hasher);
        source.hash(&mut hasher);
        destination.hash(&mut hasher);
        let hash = hasher.finish();
        let loss_threshold = (self.default_loss_rate * u64::MAX as f32) as u64;

        if hash < loss_threshold {
            // Packet lost
            return Ok(());
        }

        let delivery_time = self.clock.now() + self.default_latency;
        let packet = Packet {
            source,
            destination,
            data,
            delivery_time,
        };

        // Insert packet maintaining sorted order by delivery time
        let pos = self
            .packets_in_transit
            .iter()
            .position(|p| p.delivery_time > delivery_time)
            .unwrap_or(self.packets_in_transit.len());
        self.packets_in_transit.insert(pos, packet);

        Ok(())
    }

    pub fn tick(&mut self) {
        self.clock.tick();
        self.deliver_packets();
    }

    fn deliver_packets(&mut self) {
        let current_time = self.clock.now();

        while let Some(packet) = self.packets_in_transit.front() {
            if packet.delivery_time > current_time {
                break;
            }

            let packet = self.packets_in_transit.pop_front().unwrap();
            if let Some(node) = self.nodes.get_mut(&packet.destination) {
                if node.online {
                    node.inbox.push_back(packet.data);
                }
            }
        }
    }

    pub fn receive(&mut self, node_id: NodeId) -> Option<Vec<u8>> {
        self.nodes.get_mut(&node_id)?.inbox.pop_front()
    }

    pub fn set_latency(&mut self, latency: Duration) {
        self.default_latency = latency;
    }

    pub fn set_loss_rate(&mut self, rate: f32) {
        self.default_loss_rate = rate.clamp(0.0, 1.0);
    }
}

#[derive(Debug, thiserror::Error)]
pub enum NetworkError {
    #[error("Unknown node: {0}")]
    UnknownNode(NodeId),

    #[error("Node offline: {0}")]
    NodeOffline(NodeId),
}
