use crate::network::{NetworkError, SimNet};
use crate::topology::TopologyBuilder;
use bee_core::clock::MockClock;
use bee_core::identity::NodeId;
use ed25519_dalek::SigningKey;
use proptest::prelude::*;
use std::time::Duration;

fn create_test_node_id(seed: u8) -> NodeId {
    let mut key_bytes = [0u8; 32];
    key_bytes[0] = seed;
    let signing_key = SigningKey::from_bytes(&key_bytes);
    NodeId::from_public_key(&signing_key.verifying_key())
}

#[test]
fn simnet_delivery_follows_latency_and_loss_profile() {
    let clock = MockClock::new();
    let mut net = SimNet::new(clock);

    let node1 = create_test_node_id(1);
    let node2 = create_test_node_id(2);

    net.add_node(node1);
    net.add_node(node2);

    // Test immediate delivery with zero latency
    net.set_latency(Duration::ZERO);
    net.set_loss_rate(0.0);

    let message1 = b"test message 1".to_vec();
    net.send(node1, node2, message1.clone()).unwrap();

    // Should not be delivered yet (need to tick)
    assert_eq!(net.receive(node2), None);

    // After tick, should be delivered
    net.tick();
    assert_eq!(net.receive(node2), Some(message1));
    assert_eq!(net.receive(node2), None); // No more messages

    // Test delivery with latency
    let latency = Duration::from_millis(100);
    net.set_latency(latency);

    let message2 = b"delayed message".to_vec();
    net.send(node1, node2, message2.clone()).unwrap();

    // Should not be delivered immediately
    net.tick();
    assert_eq!(net.receive(node2), None);

    // Advance time but not enough
    for _ in 0..50 {
        net.tick(); // Default tick is 1ms
    }
    assert_eq!(net.receive(node2), None);

    // Advance past latency
    for _ in 0..50 {
        net.tick();
    }
    assert_eq!(net.receive(node2), Some(message2));

    // Test packet loss (deterministic)
    net.set_loss_rate(0.5); // 50% loss rate

    // Send multiple messages - some should be lost deterministically
    let mut received_count = 0;

    for i in 0..10 {
        let msg = vec![i; 10];
        net.send(node1, node2, msg).unwrap();
    }

    // Deliver all packets
    for _ in 0..200 {
        net.tick();
    }

    // Count received messages
    while net.receive(node2).is_some() {
        received_count += 1;
    }

    let lost_count = 10 - received_count;

    // With 50% loss rate, we should lose some packets
    assert!(lost_count > 0);
    assert!(received_count > 0);
}

#[test]
fn simnet_respects_node_presence() {
    let clock = MockClock::new();
    let mut net = SimNet::new(clock);

    let node1 = create_test_node_id(1);
    let node2 = create_test_node_id(2);

    net.add_node(node1);

    // Sending to non-existent node should fail
    let result = net.send(node1, node2, vec![1, 2, 3]);
    assert!(matches!(result, Err(NetworkError::UnknownNode(_))));

    // Sending from non-existent node should fail
    net.add_node(node2);
    let node3 = create_test_node_id(3);
    let result = net.send(node3, node2, vec![1, 2, 3]);
    assert!(matches!(result, Err(NetworkError::UnknownNode(_))));
}

#[test]
fn topology_builder_creates_correct_structures() {
    let nodes: Vec<NodeId> = (0..5).map(create_test_node_id).collect();

    // Test line topology
    let line = TopologyBuilder::line(nodes.clone());
    assert_eq!(line.len(), 4);
    assert!(line.contains(&(nodes[0], nodes[1])));
    assert!(line.contains(&(nodes[3], nodes[4])));
    assert!(!line.contains(&(nodes[0], nodes[4])));

    // Test ring topology
    let ring = TopologyBuilder::ring(nodes.clone());
    assert_eq!(ring.len(), 5);
    assert!(ring.contains(&(nodes[4], nodes[0]))); // Closing edge

    // Test grid topology (2x3 grid with 5 nodes)
    let grid = TopologyBuilder::grid(nodes.clone(), 2);
    assert!(grid.contains(&(nodes[0], nodes[1]))); // Horizontal
    assert!(grid.contains(&(nodes[0], nodes[2]))); // Vertical

    // Test full mesh
    let mesh = TopologyBuilder::full_mesh(nodes.clone());
    assert_eq!(mesh.len(), 10); // 5 choose 2 = 10 edges
}

// Property-based test for network message ordering
proptest! {
    #[test]
    fn simnet_preserves_message_order_per_pair(
        message_count in 1..100usize,
        latency_ms in 1..1000u64
    ) {
        let clock = MockClock::new();
        let mut net = SimNet::new(clock);

        let node1 = create_test_node_id(1);
        let node2 = create_test_node_id(2);

        net.add_node(node1);
        net.add_node(node2);
        net.set_latency(Duration::from_millis(latency_ms));
        net.set_loss_rate(0.0); // No loss for this test

        // Send numbered messages
        for i in 0..message_count {
            let msg = vec![i as u8];
            net.send(node1, node2, msg).unwrap();
        }

        // Advance time to deliver all messages
        for _ in 0..(latency_ms + 10) {
            net.tick();
        }

        // Receive messages and verify order
        let mut last_value = None;
        while let Some(msg) = net.receive(node2) {
            let value = msg[0];
            if let Some(last) = last_value {
                prop_assert!(value > last, "Messages arrived out of order");
            }
            last_value = Some(value);
        }

        // Verify all messages arrived
        prop_assert_eq!(last_value, Some((message_count - 1) as u8));
    }
}
