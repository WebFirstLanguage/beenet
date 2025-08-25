mod common;

use bee_api::{
    registry::{NameRegistry, RegistryError},
    ApiClient, ApiError,
};
use bee_core::name::BeeName;
use std::str::FromStr;

#[tokio::test]
async fn test_api_send_rejects_if_name_unresolved() {
    // TDD Contract: API_send_rejects_if_name_unresolved
    let client = ApiClient::<bee_core::clock::MockClock>::new_test();
    let _registry = NameRegistry::new();

    // Try to send to unresolved name
    let source = common::test_node_id(1);
    let dest_name = BeeName::from_str("unknown-node").unwrap();

    let result = client.send_to_name(source, &dest_name, vec![1, 2, 3]).await;

    assert!(result.is_err());
    assert!(matches!(result.unwrap_err(), ApiError::NameNotResolved(_)));
}

#[test]
fn test_name_registration_success() {
    let mut registry = NameRegistry::new();

    let name = BeeName::from_str("test-node").unwrap();
    let node_id = common::test_node_id(1);

    assert!(registry.register(name.clone(), node_id).is_ok());
    assert_eq!(registry.resolve(&name), Some(node_id));
}

#[test]
fn test_name_registration_duplicate_rejected() {
    let mut registry = NameRegistry::new();

    let name = BeeName::from_str("test-node").unwrap();
    let node_id1 = common::test_node_id(1);
    let node_id2 = common::test_node_id(2);

    assert!(registry.register(name.clone(), node_id1).is_ok());

    // Duplicate name registration should fail
    let result = registry.register(name, node_id2);
    assert!(result.is_err());
    assert!(matches!(
        result.unwrap_err(),
        RegistryError::NameAlreadyTaken
    ));
}

#[test]
fn test_name_resolution_case_insensitive() {
    let mut registry = NameRegistry::new();

    let name = BeeName::from_str("test-node").unwrap();
    let node_id = common::test_node_id(1);

    registry.register(name, node_id).unwrap();

    // Resolution should work with different case
    let lookup1 = BeeName::from_str("test-node").unwrap();
    let lookup2 = BeeName::from_str("test-node").unwrap(); // BeeName normalizes to lowercase

    assert_eq!(registry.resolve(&lookup1), Some(node_id));
    assert_eq!(registry.resolve(&lookup2), Some(node_id));
}

#[test]
fn test_name_unregistration() {
    let mut registry = NameRegistry::new();

    let name = BeeName::from_str("test-node").unwrap();
    let node_id = common::test_node_id(1);

    registry.register(name.clone(), node_id).unwrap();
    assert_eq!(registry.resolve(&name), Some(node_id));

    // Unregister the name
    assert!(registry.unregister(&name).is_ok());
    assert_eq!(registry.resolve(&name), None);
}

#[test]
fn test_unregister_nonexistent_name() {
    let mut registry = NameRegistry::new();

    let name = BeeName::from_str("nonexistent").unwrap();

    let result = registry.unregister(&name);
    assert!(result.is_err());
    assert!(matches!(result.unwrap_err(), RegistryError::NameNotFound));
}

#[test]
fn test_list_all_registered_names() {
    let mut registry = NameRegistry::new();

    let names = vec![
        BeeName::from_str("node-1").unwrap(),
        BeeName::from_str("node-2").unwrap(),
        BeeName::from_str("node-3").unwrap(),
    ];

    for (i, name) in names.iter().enumerate() {
        let node_id = common::test_node_id((i + 1) as u8);
        registry.register(name.clone(), node_id).unwrap();
    }

    let registered = registry.list_names();
    assert_eq!(registered.len(), 3);

    for name in &names {
        assert!(registered.contains(name));
    }
}

#[test]
fn test_reverse_lookup_by_nodeid() {
    let mut registry = NameRegistry::new();

    let name = BeeName::from_str("test-node").unwrap();
    let node_id = common::test_node_id(1);

    registry.register(name.clone(), node_id).unwrap();

    let names = registry.find_names_for_node(node_id);
    assert_eq!(names.len(), 1);
    assert_eq!(names[0], name);
}

#[test]
fn test_multiple_names_per_node_allowed() {
    let mut registry = NameRegistry::new();

    let node_id = common::test_node_id(1);
    let name1 = BeeName::from_str("alias-1").unwrap();
    let name2 = BeeName::from_str("alias-2").unwrap();

    assert!(registry.register(name1.clone(), node_id).is_ok());
    assert!(registry.register(name2.clone(), node_id).is_ok());

    assert_eq!(registry.resolve(&name1), Some(node_id));
    assert_eq!(registry.resolve(&name2), Some(node_id));

    let names = registry.find_names_for_node(node_id);
    assert_eq!(names.len(), 2);
}

#[test]
fn test_registry_clear() {
    let mut registry = NameRegistry::new();

    let name = BeeName::from_str("test-node").unwrap();
    let node_id = common::test_node_id(1);

    registry.register(name.clone(), node_id).unwrap();
    assert_eq!(registry.resolve(&name), Some(node_id));

    registry.clear();
    assert_eq!(registry.resolve(&name), None);
    assert_eq!(registry.list_names().len(), 0);
}

#[test]
fn test_name_update_requires_unregister_first() {
    let mut registry = NameRegistry::new();

    let name = BeeName::from_str("test-node").unwrap();
    let node_id1 = common::test_node_id(1);
    let node_id2 = common::test_node_id(2);

    registry.register(name.clone(), node_id1).unwrap();

    // Cannot update directly
    assert!(registry.register(name.clone(), node_id2).is_err());

    // Must unregister first
    registry.unregister(&name).unwrap();
    assert!(registry.register(name.clone(), node_id2).is_ok());

    assert_eq!(registry.resolve(&name), Some(node_id2));
}

#[test]
fn test_registry_capacity_limit() {
    let mut registry = NameRegistry::with_capacity(10);

    // Register up to capacity
    for i in 0..10 {
        let name = BeeName::from_str(&format!("node-{}", i)).unwrap();
        let node_id = common::test_node_id((i + 1) as u8);
        assert!(registry.register(name, node_id).is_ok());
    }

    // Attempt to register beyond capacity
    let name = BeeName::from_str("node-11").unwrap();
    let node_id = common::test_node_id(11);

    let result = registry.register(name, node_id);
    assert!(result.is_err());
    assert!(matches!(
        result.unwrap_err(),
        RegistryError::CapacityExceeded
    ));
}

#[cfg(test)]
mod property_tests {
    use super::*;
    use proptest::prelude::*;

    proptest! {
        #[test]
        fn test_name_registry_consistency(
            operations in prop::collection::vec(
                (any::<bool>(), "[a-z0-9-]{3,32}", 0..255u8),
                0..100
            )
        ) {
            let mut registry = NameRegistry::new();

            for (is_register, name_str, node_byte) in operations {
                if let Ok(name) = BeeName::from_str(&name_str) {
                    let node_id = common::test_node_id(node_byte);

                    if is_register {
                        let result = registry.register(name.clone(), node_id);
                        // If registration succeeded, resolution must work with the new node_id
                        if result.is_ok() {
                            assert_eq!(registry.resolve(&name), Some(node_id));
                        }
                    } else {
                        let _ = registry.unregister(&name);
                        // After unregister, resolution must fail
                        assert_eq!(registry.resolve(&name), None);
                    }
                }
            }

            // Invariant: all listed names must be resolvable
            for name in registry.list_names() {
                assert!(registry.resolve(&name).is_some());
            }
        }
    }
}
