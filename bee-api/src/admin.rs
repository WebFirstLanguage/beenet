use crate::error::ApiError;
use crate::ApiConfig;
use bee_core::callsign::{Callsign, RegulatoryBinding};
use bee_core::identity::NodeId;
use bee_core::name::BeeName;
use std::collections::HashMap;
use std::str::FromStr;
use std::time::{Duration, SystemTime};

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum RegulatoryMode {
    Part97Enabled,
    Part97Disabled,
}

pub struct AdminApi<'a> {
    config: &'a mut ApiConfig,
    name_bindings: HashMap<BeeName, NodeId>,
    regulatory_binding: Option<RegulatoryBinding>,
    transmitting: bool,
    last_id_beacon: Option<SystemTime>,
    force_id_beacon_due: bool,
}

impl<'a> AdminApi<'a> {
    pub fn new(config: &'a mut ApiConfig) -> Self {
        Self {
            config,
            name_bindings: HashMap::new(),
            regulatory_binding: None,
            transmitting: false,
            last_id_beacon: None,
            force_id_beacon_due: false,
        }
    }

    pub fn get_callsign(&self) -> Option<&Callsign> {
        self.config.callsign.as_ref()
    }

    pub fn set_callsign(&mut self, callsign: Callsign) -> Result<(), ApiError> {
        self.config.callsign = Some(callsign);
        Ok(())
    }

    pub fn get_regulatory_mode(&self) -> RegulatoryMode {
        self.config.regulatory_mode
    }

    pub fn set_regulatory_mode(&mut self, mode: RegulatoryMode) -> Result<(), ApiError> {
        // If enabling Part 97, validate that a callsign is set
        if mode == RegulatoryMode::Part97Enabled && self.config.callsign.is_none() {
            return Err(ApiError::CallsignRequired);
        }

        self.config.regulatory_mode = mode;

        // If enabling Part 97, disable encryption
        if mode == RegulatoryMode::Part97Enabled {
            self.config.encryption_enabled = false;
        }

        Ok(())
    }

    pub fn get_regulatory_binding(&self) -> Option<&RegulatoryBinding> {
        self.regulatory_binding.as_ref()
    }

    pub fn set_regulatory_binding(&mut self, binding: RegulatoryBinding) -> Result<(), ApiError> {
        self.config.callsign = Some(binding.callsign.clone());
        self.regulatory_binding = Some(binding);
        Ok(())
    }

    pub fn validate_part97_requirements(&self) -> Result<(), ApiError> {
        if self.config.is_part97_enabled() && self.config.callsign.is_none() {
            return Err(ApiError::CallsignRequired);
        }
        Ok(())
    }

    pub fn enable_encryption(&mut self) -> Result<(), ApiError> {
        if self.config.is_part97_enabled() {
            return Err(ApiError::EncryptionNotAllowedInPart97);
        }
        self.config.encryption_enabled = true;
        Ok(())
    }

    pub fn disable_encryption(&mut self) -> Result<(), ApiError> {
        self.config.encryption_enabled = false;
        Ok(())
    }

    pub fn is_encryption_enabled(&self) -> bool {
        self.config.encryption_enabled
    }

    pub fn id_beacon_required(&self) -> bool {
        self.config.is_part97_enabled()
    }

    pub fn id_beacon_interval(&self) -> Duration {
        Duration::from_secs(600) // 10 minutes
    }

    pub fn mark_transmission_start(&mut self) -> Result<(), ApiError> {
        self.transmitting = true;
        Ok(())
    }

    pub fn mark_transmission_end(&mut self) -> Result<bool, ApiError> {
        self.transmitting = false;
        Ok(self.config.is_part97_enabled())
    }

    pub fn is_transmitting(&self) -> bool {
        self.transmitting
    }

    pub fn mark_id_beacon_sent(&mut self) -> Result<(), ApiError> {
        self.last_id_beacon = Some(SystemTime::now());
        self.force_id_beacon_due = false;
        Ok(())
    }

    pub fn is_id_beacon_due(&self) -> bool {
        if self.force_id_beacon_due {
            return true;
        }

        if let Some(last) = self.last_id_beacon {
            let elapsed = SystemTime::now()
                .duration_since(last)
                .unwrap_or(Duration::ZERO);
            elapsed >= self.id_beacon_interval()
        } else {
            true
        }
    }

    pub fn test_force_id_beacon_due(&mut self) {
        self.force_id_beacon_due = true;
    }

    pub fn set_beename_binding(&mut self, name: BeeName, node_id: NodeId) -> Result<(), ApiError> {
        self.name_bindings.insert(name, node_id);
        Ok(())
    }

    pub fn get_beename_binding(&self, name: &BeeName) -> Option<NodeId> {
        self.name_bindings.get(name).copied()
    }

    pub fn clear_configuration(&mut self) -> Result<(), ApiError> {
        self.config.callsign = None;
        self.name_bindings.clear();
        self.regulatory_binding = None;
        Ok(())
    }

    pub fn get_swarm_id(&self) -> [u8; 8] {
        self.config.swarm_id
    }

    pub fn set_swarm_id(&mut self, swarm_id: [u8; 8]) -> Result<(), ApiError> {
        self.config.swarm_id = swarm_id;
        Ok(())
    }

    pub fn get_message_timeout(&self) -> Duration {
        self.config.message_timeout
    }

    pub fn set_message_timeout(&mut self, timeout: Duration) -> Result<(), ApiError> {
        self.config.message_timeout = timeout;
        Ok(())
    }

    pub fn get_max_queue_size(&self) -> usize {
        self.config.max_queue_size
    }

    pub fn set_max_queue_size(&mut self, size: usize) -> Result<(), ApiError> {
        if size == 0 {
            return Err(ApiError::InvalidConfiguration(
                "Queue size must be > 0".into(),
            ));
        }
        self.config.max_queue_size = size;
        Ok(())
    }

    pub fn export_configuration(&self) -> HashMap<String, serde_json::Value> {
        use serde_json::json;

        let mut config = HashMap::new();

        config.insert(
            "callsign".to_string(),
            self.config
                .callsign
                .as_ref()
                .map(|c| json!(c.as_str()))
                .unwrap_or(json!(null)),
        );

        config.insert(
            "regulatory_mode".to_string(),
            json!(match self.config.regulatory_mode {
                RegulatoryMode::Part97Enabled => "part97_enabled",
                RegulatoryMode::Part97Disabled => "part97_disabled",
            }),
        );

        config.insert(
            "encryption_enabled".to_string(),
            json!(self.config.encryption_enabled),
        );

        config.insert(
            "swarm_id".to_string(),
            json!(hex::encode(self.config.swarm_id)),
        );

        config
    }

    pub fn import_configuration(
        &mut self,
        config: HashMap<String, serde_json::Value>,
    ) -> Result<(), ApiError> {
        if let Some(callsign_val) = config.get("callsign") {
            if let Some(callsign_str) = callsign_val.as_str() {
                self.config.callsign = Some(
                    Callsign::from_str(callsign_str)
                        .map_err(|e| ApiError::InvalidConfiguration(e.to_string()))?,
                );
            }
        }

        if let Some(mode_val) = config.get("regulatory_mode") {
            if let Some(mode_str) = mode_val.as_str() {
                self.config.regulatory_mode = match mode_str {
                    "part97_enabled" => RegulatoryMode::Part97Enabled,
                    "part97_disabled" => RegulatoryMode::Part97Disabled,
                    _ => {
                        return Err(ApiError::InvalidConfiguration(
                            "Invalid regulatory mode".into(),
                        ))
                    }
                };
            }
        }

        if let Some(enc_val) = config.get("encryption_enabled") {
            if let Some(enabled) = enc_val.as_bool() {
                self.config.encryption_enabled = enabled;
            }
        }

        Ok(())
    }
}
