mod common;

use bee_api::{
    admin::{AdminApi, RegulatoryMode},
    ApiClient, ApiConfig, ApiError,
};
use bee_core::callsign::{Callsign, RegulatoryBinding};
use bee_core::name::BeeName;
use std::str::FromStr;

#[test]
fn test_api_part97_default_is_enabled_on_radio_profiles() {
    // TDD Contract: API_part97_default_is_enabled_on_radio_profiles
    let config = ApiConfig::new_radio_profile();

    assert_eq!(config.regulatory_mode(), RegulatoryMode::Part97Enabled);
    assert!(config.is_part97_enabled());

    // Verify encryption is disabled
    assert!(!config.encryption_allowed());

    // Verify callsign is required
    assert!(config.callsign_required());
}

#[test]
fn test_part97_mode_requires_callsign() {
    let mut config = ApiConfig::new_radio_profile();
    let mut admin = AdminApi::new(&mut config);

    // Part 97 enabled by default, must have callsign
    let result = admin.validate_part97_requirements();
    assert!(result.is_err());
    assert!(matches!(result.unwrap_err(), ApiError::CallsignRequired));

    // Set callsign
    let callsign = Callsign::from_str("K7TEST").unwrap();
    admin.set_callsign(callsign).unwrap();

    // Now validation should pass
    assert!(admin.validate_part97_requirements().is_ok());
}

#[test]
fn test_part97_mode_disables_encryption() {
    let mut config = ApiConfig::new_radio_profile();
    let mut admin = AdminApi::new(&mut config);

    // Set callsign to satisfy requirements
    let callsign = Callsign::from_str("K7TEST").unwrap();
    admin.set_callsign(callsign).unwrap();

    // Try to enable encryption - should fail
    let result = admin.enable_encryption();
    assert!(result.is_err());
    assert!(matches!(
        result.unwrap_err(),
        ApiError::EncryptionNotAllowedInPart97
    ));
}

#[test]
fn test_toggle_part97_mode() {
    let mut config = ApiConfig::new();
    let mut admin = AdminApi::new(&mut config);

    // Start with Part 97 disabled (non-radio profile)
    assert_eq!(admin.get_regulatory_mode(), RegulatoryMode::Part97Disabled);

    // Set callsign first to enable Part 97
    let callsign = Callsign::from_str("K7TEST").unwrap();
    admin.set_callsign(callsign).unwrap();

    // Enable Part 97
    admin
        .set_regulatory_mode(RegulatoryMode::Part97Enabled)
        .unwrap();
    assert_eq!(admin.get_regulatory_mode(), RegulatoryMode::Part97Enabled);

    // Disable Part 97
    admin
        .set_regulatory_mode(RegulatoryMode::Part97Disabled)
        .unwrap();
    assert_eq!(admin.get_regulatory_mode(), RegulatoryMode::Part97Disabled);
}

#[test]
fn test_set_regulatory_mode_requires_callsign_for_part97() {
    let mut config = ApiConfig::new();
    let mut admin = AdminApi::new(&mut config);

    // Initially no callsign
    assert_eq!(admin.get_callsign(), None);

    // Try to enable Part 97 without a callsign - should fail
    let result = admin.set_regulatory_mode(RegulatoryMode::Part97Enabled);
    assert!(result.is_err());
    assert!(matches!(result.unwrap_err(), ApiError::CallsignRequired));

    // Mode should remain disabled
    assert_eq!(admin.get_regulatory_mode(), RegulatoryMode::Part97Disabled);

    // Set a callsign
    let callsign = Callsign::from_str("W1AW").unwrap();
    admin.set_callsign(callsign).unwrap();

    // Now enabling Part 97 should succeed
    let result = admin.set_regulatory_mode(RegulatoryMode::Part97Enabled);
    assert!(result.is_ok());
    assert_eq!(admin.get_regulatory_mode(), RegulatoryMode::Part97Enabled);
}

#[test]
fn test_set_regulatory_mode_can_always_disable_part97() {
    let mut config = ApiConfig::new();
    let mut admin = AdminApi::new(&mut config);

    // No callsign set
    assert_eq!(admin.get_callsign(), None);

    // Should be able to disable Part 97 even without a callsign
    let result = admin.set_regulatory_mode(RegulatoryMode::Part97Disabled);
    assert!(result.is_ok());
    assert_eq!(admin.get_regulatory_mode(), RegulatoryMode::Part97Disabled);

    // Set callsign and enable Part 97
    let callsign = Callsign::from_str("VE3ABC").unwrap();
    admin.set_callsign(callsign).unwrap();
    admin
        .set_regulatory_mode(RegulatoryMode::Part97Enabled)
        .unwrap();

    // Should be able to disable Part 97
    let result = admin.set_regulatory_mode(RegulatoryMode::Part97Disabled);
    assert!(result.is_ok());
    assert_eq!(admin.get_regulatory_mode(), RegulatoryMode::Part97Disabled);
}

#[test]
fn test_callsign_binding_with_beename() {
    let mut config = ApiConfig::new_radio_profile();
    let mut admin = AdminApi::new(&mut config);

    let callsign = Callsign::from_str("K7TEST").unwrap();
    let beename = BeeName::from_str("test-node").unwrap();
    let node_id = common::test_node_id(1);

    // Create regulatory binding
    let binding = RegulatoryBinding::new(callsign.clone(), beename.clone(), node_id);
    admin.set_regulatory_binding(binding.clone()).unwrap();

    // Verify binding is stored
    assert_eq!(admin.get_callsign(), Some(&callsign));
    assert_eq!(admin.get_regulatory_binding(), Some(&binding));
}

#[test]
fn test_id_beacon_required_in_part97() {
    let mut config = ApiConfig::new_radio_profile();
    let mut admin = AdminApi::new(&mut config);

    let callsign = Callsign::from_str("K7TEST").unwrap();
    admin.set_callsign(callsign).unwrap();

    // Check ID beacon requirements
    assert!(admin.id_beacon_required());
    assert_eq!(
        admin.id_beacon_interval(),
        std::time::Duration::from_secs(600)
    ); // 10 minutes
}

#[tokio::test]
async fn test_plain_text_enforcement_in_part97() {
    let config = ApiConfig::new_radio_profile();
    let client = ApiClient::<bee_core::clock::MockClock>::with_config(config);
    // Encrypted payload should be rejected
    let encrypted_payload = vec![0xFF, 0xFE, 0xFD]; // Obviously encrypted
    let result = client.validate_payload_for_part97(&encrypted_payload).await;

    assert!(result.is_err());
    assert!(matches!(
        result.unwrap_err(),
        ApiError::EncryptedPayloadInPart97
    ));

    // Plain text should be accepted
    let plain_text = b"Hello, this is plain text".to_vec();
    assert!(client
        .validate_payload_for_part97(&plain_text)
        .await
        .is_ok());
}

#[test]
fn test_non_radio_profile_allows_encryption() {
    let mut config = ApiConfig::new(); // Non-radio profile
    let mut admin = AdminApi::new(&mut config);

    // Part 97 disabled by default for non-radio
    assert_eq!(admin.get_regulatory_mode(), RegulatoryMode::Part97Disabled);

    // Encryption should be allowed
    assert!(admin.enable_encryption().is_ok());
    assert!(admin.is_encryption_enabled());
}

#[test]
fn test_switching_to_part97_disables_existing_encryption() {
    let mut config = ApiConfig::new(); // Non-radio profile
    let mut admin = AdminApi::new(&mut config);

    // Enable encryption
    admin.enable_encryption().unwrap();
    assert!(admin.is_encryption_enabled());

    // Set callsign first (required for Part 97)
    let callsign = Callsign::from_str("K7TEST").unwrap();
    admin.set_callsign(callsign).unwrap();

    // Switch to Part 97 mode
    admin
        .set_regulatory_mode(RegulatoryMode::Part97Enabled)
        .unwrap();

    // Encryption should be automatically disabled
    assert!(!admin.is_encryption_enabled());
}

#[test]
fn test_callsign_validation_in_api() {
    let mut config = ApiConfig::new_radio_profile();
    let mut admin = AdminApi::new(&mut config);

    // Valid callsigns
    assert!(admin
        .set_callsign(Callsign::from_str("K7TEST").unwrap())
        .is_ok());
    assert!(admin
        .set_callsign(Callsign::from_str("W1AW").unwrap())
        .is_ok());
    assert!(admin
        .set_callsign(Callsign::from_str("VE3ABC").unwrap())
        .is_ok());

    // Invalid callsigns should have been rejected by Callsign::from_str
    assert!(Callsign::from_str("invalid!").is_err());
    assert!(Callsign::from_str("").is_err());
    assert!(Callsign::from_str("a").is_err()); // Too short
}

#[test]
fn test_id_beacon_end_of_transmission() {
    let mut config = ApiConfig::new_radio_profile();
    let mut admin = AdminApi::new(&mut config);

    let callsign = Callsign::from_str("K7TEST").unwrap();
    admin.set_callsign(callsign).unwrap();

    // Start transmission
    admin.mark_transmission_start().unwrap();
    assert!(admin.is_transmitting());

    // End transmission - should trigger ID beacon requirement
    let beacon_required = admin.mark_transmission_end().unwrap();
    assert!(beacon_required);
    assert!(!admin.is_transmitting());
}

#[test]
fn test_periodic_id_beacon_tracking() {
    let mut config = ApiConfig::new_radio_profile();
    let mut admin = AdminApi::new(&mut config);

    let callsign = Callsign::from_str("K7TEST").unwrap();
    admin.set_callsign(callsign).unwrap();

    // Check if ID beacon is due
    let clock = bee_core::clock::MockClock::new();
    admin.mark_id_beacon_sent(&clock).unwrap();
    assert!(!admin.is_id_beacon_due(&clock));

    // Simulate time passing (would use virtual clock in real implementation)
    // For testing, manually mark as due
    admin.test_force_id_beacon_due();
    assert!(admin.is_id_beacon_due(&clock));
}

#[cfg(test)]
mod property_tests {
    use super::*;
    use proptest::prelude::*;

    proptest! {
        #[test]
        fn test_part97_mode_invariants(
            enable_part97 in any::<bool>(),
            enable_encryption in any::<bool>(),
            set_callsign in any::<bool>(),
        ) {
            let mut config = ApiConfig::new();
            let mut admin = AdminApi::new(&mut config);

            if enable_part97 {
                // Must set callsign before enabling Part 97
                if set_callsign {
                    let callsign = Callsign::from_str("K7TEST").unwrap();
                    admin.set_callsign(callsign).unwrap();

                    // Now we can enable Part 97
                    admin.set_regulatory_mode(RegulatoryMode::Part97Enabled).unwrap();

                    // Invariant: Part 97 mode never allows encryption
                    if enable_encryption {
                        assert!(admin.enable_encryption().is_err());
                    }
                    assert!(!admin.is_encryption_enabled());

                    // Invariant: Part 97 mode requires callsign (which we have)
                    assert!(admin.validate_part97_requirements().is_ok());
                } else {
                    // Try to enable Part 97 without callsign - should fail
                    let result = admin.set_regulatory_mode(RegulatoryMode::Part97Enabled);
                    assert!(result.is_err());
                    assert!(matches!(result.unwrap_err(), ApiError::CallsignRequired));

                    // Mode should remain disabled
                    assert_eq!(admin.get_regulatory_mode(), RegulatoryMode::Part97Disabled);
                }
            } else {
                admin.set_regulatory_mode(RegulatoryMode::Part97Disabled).unwrap();

                // Non-Part 97 mode allows encryption
                if enable_encryption {
                    assert!(admin.enable_encryption().is_ok());
                    assert!(admin.is_encryption_enabled());
                }

                // Callsign not required
                assert!(admin.validate_part97_requirements().is_ok());
            }
        }
    }
}
