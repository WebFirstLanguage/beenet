mod common;

use bee_api::{
    admin::{AdminApi, RegulatoryMode},
    ApiConfig,
};
use bee_core::callsign::Callsign;
use bee_core::name::BeeName;
use std::str::FromStr;

#[test]
fn test_set_get_callsign() {
    let mut config = ApiConfig::new();
    let mut admin = AdminApi::new(&mut config);
    
    // Initially no callsign
    assert_eq!(admin.get_callsign(), None);
    
    // Set callsign
    let callsign = Callsign::from_str("K7TEST").unwrap();
    assert!(admin.set_callsign(callsign.clone()).is_ok());
    
    // Get callsign
    assert_eq!(admin.get_callsign(), Some(&callsign));
}

#[test]
fn test_update_callsign() {
    let mut config = ApiConfig::new();
    let mut admin = AdminApi::new(&mut config);
    
    // Set initial callsign
    let callsign1 = Callsign::from_str("K7TEST").unwrap();
    admin.set_callsign(callsign1.clone()).unwrap();
    assert_eq!(admin.get_callsign(), Some(&callsign1));
    
    // Update callsign
    let callsign2 = Callsign::from_str("W1AW").unwrap();
    admin.set_callsign(callsign2.clone()).unwrap();
    assert_eq!(admin.get_callsign(), Some(&callsign2));
}

#[test]
fn test_toggle_regulatory_mode() {
    let mut config = ApiConfig::new();
    let mut admin = AdminApi::new(&mut config);
    
    // Check initial state
    let initial = admin.get_regulatory_mode();
    
    // Toggle to opposite state
    let new_mode = if initial == RegulatoryMode::Part97Enabled {
        RegulatoryMode::Part97Disabled
    } else {
        RegulatoryMode::Part97Enabled
    };
    
    assert!(admin.set_regulatory_mode(new_mode).is_ok());
    assert_eq!(admin.get_regulatory_mode(), new_mode);
}

#[test]
fn test_set_beename_binding() {
    let mut config = ApiConfig::new();
    let mut admin = AdminApi::new(&mut config);
    
    let beename = BeeName::from_str("test-node").unwrap();
    let node_id = common::test_node_id(1);
    
    // Set binding
    assert!(admin.set_beename_binding(beename.clone(), node_id).is_ok());
    
    // Verify binding
    assert_eq!(admin.get_beename_binding(&beename), Some(node_id));
}

#[test]
fn test_clear_configuration() {
    let mut config = ApiConfig::new();
    let mut admin = AdminApi::new(&mut config);
    
    // Set various configuration
    let callsign = Callsign::from_str("K7TEST").unwrap();
    admin.set_callsign(callsign).unwrap();
    admin.set_regulatory_mode(RegulatoryMode::Part97Enabled).unwrap();
    
    let beename = BeeName::from_str("test-node").unwrap();
    let node_id = common::test_node_id(1);
    admin.set_beename_binding(beename.clone(), node_id).unwrap();
    
    // Clear configuration
    admin.clear_configuration().unwrap();
    
    // Verify everything is cleared
    assert_eq!(admin.get_callsign(), None);
    assert_eq!(admin.get_beename_binding(&beename), None);
}

#[test]
fn test_swarm_id_configuration() {
    let mut config = ApiConfig::new();
    let mut admin = AdminApi::new(&mut config);
    
    // Default swarm ID
    let default_swarm = admin.get_swarm_id();
    assert_eq!(default_swarm.len(), 8);
    
    // Set custom swarm ID
    let new_swarm = [0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88];
    assert!(admin.set_swarm_id(new_swarm).is_ok());
    assert_eq!(admin.get_swarm_id(), new_swarm);
}

#[test]
fn test_encryption_toggle() {
    let mut config = ApiConfig::new();
    let mut admin = AdminApi::new(&mut config);
    
    // Ensure Part 97 is disabled for this test
    admin.set_regulatory_mode(RegulatoryMode::Part97Disabled).unwrap();
    
    // Initially encryption disabled
    assert!(!admin.is_encryption_enabled());
    
    // Enable encryption
    assert!(admin.enable_encryption().is_ok());
    assert!(admin.is_encryption_enabled());
    
    // Disable encryption
    assert!(admin.disable_encryption().is_ok());
    assert!(!admin.is_encryption_enabled());
}

#[test]
fn test_message_timeout_configuration() {
    let mut config = ApiConfig::new();
    let mut admin = AdminApi::new(&mut config);
    
    // Default timeout
    let default_timeout = admin.get_message_timeout();
    assert_eq!(default_timeout, std::time::Duration::from_secs(300)); // 5 minutes
    
    // Set custom timeout
    let new_timeout = std::time::Duration::from_secs(600);
    assert!(admin.set_message_timeout(new_timeout).is_ok());
    assert_eq!(admin.get_message_timeout(), new_timeout);
}

#[test]
fn test_queue_size_limit() {
    let mut config = ApiConfig::new();
    let mut admin = AdminApi::new(&mut config);
    
    // Default queue size
    let default_size = admin.get_max_queue_size();
    assert_eq!(default_size, 1000);
    
    // Set custom queue size
    assert!(admin.set_max_queue_size(500).is_ok());
    assert_eq!(admin.get_max_queue_size(), 500);
    
    // Reject invalid size
    assert!(admin.set_max_queue_size(0).is_err());
}

#[test]
fn test_export_configuration() {
    let mut config = ApiConfig::new();
    let mut admin = AdminApi::new(&mut config);
    
    // Set up configuration
    let callsign = Callsign::from_str("K7TEST").unwrap();
    admin.set_callsign(callsign.clone()).unwrap();
    admin.set_regulatory_mode(RegulatoryMode::Part97Enabled).unwrap();
    admin.enable_encryption().unwrap_or(()); // May fail if Part97 enabled
    
    // Export configuration
    let exported = admin.export_configuration();
    
    assert!(exported.contains_key("callsign"));
    assert!(exported.contains_key("regulatory_mode"));
    assert!(exported.contains_key("encryption_enabled"));
    assert!(exported.contains_key("swarm_id"));
}

#[test]
fn test_import_configuration() {
    let mut config1 = ApiConfig::new();
    let mut admin1 = AdminApi::new(&mut config1);
    
    // Set up configuration
    let callsign = Callsign::from_str("K7TEST").unwrap();
    admin1.set_callsign(callsign.clone()).unwrap();
    admin1.set_regulatory_mode(RegulatoryMode::Part97Disabled).unwrap();
    
    // Export
    let exported = admin1.export_configuration();
    
    // Import into new config
    let mut config2 = ApiConfig::new();
    let mut admin2 = AdminApi::new(&mut config2);
    
    assert!(admin2.import_configuration(exported).is_ok());
    
    // Verify imported configuration
    assert_eq!(admin2.get_callsign(), Some(&callsign));
    assert_eq!(admin2.get_regulatory_mode(), RegulatoryMode::Part97Disabled);
}