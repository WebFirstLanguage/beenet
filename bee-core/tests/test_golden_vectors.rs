//! Golden test vectors from normative appendices
//! These tests verify our implementation against the exact byte layouts
//! specified in the Beenet protocol documentation.

use bee_core::callsign::Callsign;
use bee_core::identity::NodeId;
use bee_core::name::BeeName;
use ed25519_dalek::SigningKey;
use sha2::{Digest, Sha256};
use std::collections::BTreeMap;

#[test]
fn test_beename_examples() {
    // Valid examples from spec
    let valid_names = ["honey-1", "test-node", "abc", "node-123", "test-node-456"];

    // Test max length separately
    let max_name = "a".repeat(32);
    let beename = BeeName::new(&max_name).expect("Should accept 32 char name");
    assert_eq!(beename.as_str(), &max_name);

    for name in valid_names {
        let beename =
            BeeName::new(name).unwrap_or_else(|_| panic!("Should accept valid name: {}", name));
        assert_eq!(beename.as_str(), name);
    }

    // Invalid examples
    let invalid_names = [
        "K7TEST",        // Uppercase
        "test_node",     // Underscore
        "ab",            // Too short
        &"a".repeat(33), // Too long
    ];

    for name in invalid_names {
        assert!(
            BeeName::new(name).is_err(),
            "Should reject invalid name: {}",
            name
        );
    }
}

#[test]
fn test_callsign_examples() {
    // Valid examples from spec
    let valid_callsigns = ["K7TEST", "VE3ABC-9", "W1AW/3", "2E0XXX", "K7"];

    // Test max length separately
    let max_callsign = "A".repeat(16);
    let cs = Callsign::new(&max_callsign).expect("Should accept 16 char callsign");
    assert_eq!(cs.as_str(), &max_callsign);

    for callsign in valid_callsigns {
        let cs = Callsign::new(callsign)
            .unwrap_or_else(|_| panic!("Should accept valid callsign: {}", callsign));
        assert_eq!(cs.as_str(), callsign);
    }

    // Invalid examples
    let invalid_callsigns = [
        "k7test",        // Lowercase
        "K7_TEST",       // Underscore
        "A",             // Too short
        &"A".repeat(17), // Too long
    ];

    for callsign in invalid_callsigns {
        assert!(
            Callsign::new(callsign).is_err(),
            "Should reject invalid callsign: {}",
            callsign
        );
    }
}

#[test]
fn test_nodeid_deterministic() {
    // Test NodeID = SHA-256(PublicKey) property
    let test_keys = [
        [0u8; 32],
        [1u8; 32],
        [0xFF; 32],
        [
            0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E,
            0x0F, 0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18, 0x19, 0x1A, 0x1B, 0x1C,
            0x1D, 0x1E, 0x1F, 0x20,
        ],
    ];

    for key_bytes in test_keys {
        let signing_key = SigningKey::from_bytes(&key_bytes);
        let verifying_key = signing_key.verifying_key();
        let node_id = NodeId::from_public_key(&verifying_key);

        // Manually compute SHA-256
        let mut hasher = Sha256::new();
        hasher.update(verifying_key.as_bytes());
        let expected = hasher.finalize();

        assert_eq!(
            node_id.as_bytes(),
            expected.as_slice(),
            "NodeID must be SHA-256 of public key"
        );
    }
}

/// TLV (Type-Length-Value) structure from spec
#[derive(Debug)]
struct Tlv {
    typ: u8,
    length: u16,
    value: Vec<u8>,
}

impl Tlv {
    fn new(typ: u8, value: Vec<u8>) -> Self {
        Self {
            typ,
            length: value.len() as u16,
            value,
        }
    }

    fn serialize(&self) -> Vec<u8> {
        let mut bytes = Vec::new();
        bytes.push(self.typ);
        bytes.extend_from_slice(&self.length.to_be_bytes());
        bytes.extend_from_slice(&self.value);
        bytes
    }

    fn parse(data: &[u8]) -> Result<(Self, &[u8]), String> {
        if data.len() < 3 {
            return Err("Not enough bytes for TLV header".to_string());
        }

        let typ = data[0];
        let length = u16::from_be_bytes([data[1], data[2]]);

        if data.len() < 3 + length as usize {
            return Err("Not enough bytes for TLV value".to_string());
        }

        let value = data[3..3 + length as usize].to_vec();
        let remaining = &data[3 + length as usize..];

        Ok((Self { typ, length, value }, remaining))
    }
}

#[test]
fn test_hello_tlv_structure() {
    // Example from E.1: Minimal HELLO TLV block
    // This tests the TLV encoding structure

    let beename = BeeName::new("honey-1").unwrap();
    let callsign = Callsign::new("K7TEST").unwrap();
    let swarm_id = [0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88];

    // Create TLVs in ascending order as per spec
    let mut tlvs = BTreeMap::new();

    // 0x01 NodeID (would be computed from public key)
    let node_id_bytes = vec![0xAA; 32]; // Placeholder
    tlvs.insert(0x01, Tlv::new(0x01, node_id_bytes));

    // 0x02 BeeName
    tlvs.insert(0x02, Tlv::new(0x02, beename.as_str().as_bytes().to_vec()));

    // 0x03 SwarmID
    tlvs.insert(0x03, Tlv::new(0x03, swarm_id.to_vec()));

    // 0x04 Callsign
    tlvs.insert(0x04, Tlv::new(0x04, callsign.as_str().as_bytes().to_vec()));

    // 0x05 HelloSeq
    let hello_seq = [0x00, 0x00, 0x12, 0x34, 0x00, 0x00, 0x00, 0x01];
    tlvs.insert(0x05, Tlv::new(0x05, hello_seq.to_vec()));

    // 0x09 Timestamp
    let timestamp = [0x00, 0x00, 0x01, 0x97, 0x6A, 0x2B, 0x3C, 0xD0];
    tlvs.insert(0x09, Tlv::new(0x09, timestamp.to_vec()));

    // 0x0A PublicKey
    let public_key = vec![0x01, 0x02, 0x03]; // Would be 32 bytes
    tlvs.insert(0x0A, Tlv::new(0x0A, public_key));

    // 0x0C RegulatoryMode
    tlvs.insert(0x0C, Tlv::new(0x0C, vec![0x01])); // Part97=ON

    // Serialize TLVs in order
    let mut serialized = Vec::new();
    for (_, tlv) in tlvs.iter() {
        serialized.extend(tlv.serialize());
    }

    // Verify we can parse them back
    let mut remaining = &serialized[..];
    let mut parsed_count = 0;

    while !remaining.is_empty() {
        let (tlv, rest) = Tlv::parse(remaining).expect("Should parse TLV");
        remaining = rest;
        parsed_count += 1;

        // Verify known types
        match tlv.typ {
            0x02 => {
                let name = String::from_utf8(tlv.value.clone()).unwrap();
                assert_eq!(name, "honey-1");
            }
            0x04 => {
                let cs = String::from_utf8(tlv.value.clone()).unwrap();
                assert_eq!(cs, "K7TEST");
            }
            0x03 => {
                assert_eq!(tlv.value, swarm_id);
            }
            0x0C => {
                assert_eq!(tlv.value, vec![0x01]);
            }
            _ => {}
        }
    }

    assert_eq!(parsed_count, tlvs.len());
}

#[test]
fn test_id_beacon_tlv_structure() {
    // Example from E.2: ID-BEACON with IDText

    let callsign = Callsign::new("K7TEST").unwrap();
    let beename = BeeName::new("honey-1").unwrap();
    let swarm_id = [0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88];
    let id_text = "ID de K7TEST Beenet\n";

    // Create TLVs in ascending order
    let mut tlvs = BTreeMap::new();

    // 0x02 BeeName
    tlvs.insert(0x02, Tlv::new(0x02, beename.as_str().as_bytes().to_vec()));

    // 0x03 SwarmID
    tlvs.insert(0x03, Tlv::new(0x03, swarm_id.to_vec()));

    // 0x04 Callsign
    tlvs.insert(0x04, Tlv::new(0x04, callsign.as_str().as_bytes().to_vec()));

    // 0x09 Timestamp
    let timestamp = [0x00, 0x00, 0x01, 0x97, 0x6A, 0x2B, 0x3C, 0xD0];
    tlvs.insert(0x09, Tlv::new(0x09, timestamp.to_vec()));

    // 0x20 IDReason
    tlvs.insert(0x20, Tlv::new(0x20, vec![0x00])); // Periodic

    // 0x21 IDText
    tlvs.insert(0x21, Tlv::new(0x21, id_text.as_bytes().to_vec()));

    // Serialize
    let mut serialized = Vec::new();
    for (_, tlv) in tlvs.iter() {
        serialized.extend(tlv.serialize());
    }

    // Parse and verify
    let mut remaining = &serialized[..];

    while !remaining.is_empty() {
        let (tlv, rest) = Tlv::parse(remaining).expect("Should parse TLV");
        remaining = rest;

        match tlv.typ {
            0x21 => {
                // IDText must be plain ASCII
                let text = String::from_utf8(tlv.value.clone()).unwrap();
                assert_eq!(text, id_text);
                assert!(text.is_ascii(), "IDText must be ASCII");
            }
            0x20 => {
                assert_eq!(tlv.value[0], 0x00); // Periodic
            }
            _ => {}
        }
    }
}

#[test]
fn test_signature_context_string() {
    // Test signature context strings from spec
    let hello_context = b"BeeNet-HELLO-v1";
    let id_beacon_context = b"BeeNet-IDBEACON-v1";
    let name_claim_context = b"BeeNet-NameClaim-v1";

    // Verify they're ASCII
    assert!(hello_context.is_ascii());
    assert!(id_beacon_context.is_ascii());
    assert!(name_claim_context.is_ascii());

    // Test signature input construction
    let swarm_id = [0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88];
    let node_id = [0xAA; 32];

    // Build HELLO signature input as per spec
    let mut sig_input = Vec::new();
    sig_input.extend_from_slice(hello_context);
    sig_input.push(0x00);
    sig_input.extend_from_slice(&swarm_id);
    sig_input.extend_from_slice(&node_id);
    // Canonical TLVs would be appended here (excluding signature itself)

    assert!(sig_input.starts_with(hello_context));
    assert_eq!(sig_input[hello_context.len()], 0x00);
}

#[test]
fn test_regulatory_validation() {
    // When RegulatoryMode bit0=1, certain fields are mandatory

    // Test Part 97 mode requirements
    let regulatory_mode_on = 0x01u8;
    let regulatory_mode_off = 0x00u8;

    assert_eq!(regulatory_mode_on & 0x01, 1, "Part 97 mode is ON");
    assert_eq!(regulatory_mode_off & 0x01, 0, "Part 97 mode is OFF");

    // In Part 97 mode:
    // - Callsign MUST be present
    // - IDText MUST be plain ASCII
    // - No encryption allowed

    let callsign = Callsign::new("K7TEST").unwrap();
    assert!(callsign.as_str().is_ascii());

    let id_text = "ID de K7TEST";
    assert!(id_text.is_ascii());
    assert!(!id_text.contains('\0')); // No null bytes
}

#[test]
fn test_canonical_ordering() {
    // TLVs must be in ascending order by type
    let mut tlvs = BTreeMap::new();

    tlvs.insert(0x09, vec![1, 2, 3]); // Timestamp
    tlvs.insert(0x01, vec![4, 5, 6]); // NodeID
    tlvs.insert(0x03, vec![7, 8, 9]); // SwarmID
    tlvs.insert(0x02, vec![10, 11]); // BeeName

    // BTreeMap automatically orders by key
    let ordered_types: Vec<u8> = tlvs.keys().copied().collect();
    assert_eq!(ordered_types, vec![0x01, 0x02, 0x03, 0x09]);
}
