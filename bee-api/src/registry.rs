use bee_core::identity::NodeId;
use bee_core::name::BeeName;
use std::collections::HashMap;
use thiserror::Error;

#[derive(Debug, Error)]
pub enum RegistryError {
    #[error("Name already taken")]
    NameAlreadyTaken,

    #[error("Name not found")]
    NameNotFound,

    #[error("Registry capacity exceeded")]
    CapacityExceeded,
}

pub struct NameRegistry {
    name_to_node: HashMap<BeeName, NodeId>,
    node_to_names: HashMap<NodeId, Vec<BeeName>>,
    capacity: Option<usize>,
}

impl Default for NameRegistry {
    fn default() -> Self {
        Self::new()
    }
}

impl NameRegistry {
    pub fn new() -> Self {
        Self {
            name_to_node: HashMap::new(),
            node_to_names: HashMap::new(),
            capacity: None,
        }
    }

    pub fn with_capacity(capacity: usize) -> Self {
        Self {
            name_to_node: HashMap::new(),
            node_to_names: HashMap::new(),
            capacity: Some(capacity),
        }
    }

    pub fn register(&mut self, name: BeeName, node_id: NodeId) -> Result<(), RegistryError> {
        // Check capacity
        if let Some(cap) = self.capacity {
            if self.name_to_node.len() >= cap {
                return Err(RegistryError::CapacityExceeded);
            }
        }

        // Check if name is already taken
        if self.name_to_node.contains_key(&name) {
            return Err(RegistryError::NameAlreadyTaken);
        }

        // Register the name
        self.name_to_node.insert(name.clone(), node_id);

        // Update reverse mapping
        self.node_to_names.entry(node_id).or_default().push(name);

        Ok(())
    }

    pub fn unregister(&mut self, name: &BeeName) -> Result<(), RegistryError> {
        // Remove from name -> node mapping
        let node_id = self
            .name_to_node
            .remove(name)
            .ok_or(RegistryError::NameNotFound)?;

        // Remove from reverse mapping
        if let Some(names) = self.node_to_names.get_mut(&node_id) {
            names.retain(|n| n != name);
            if names.is_empty() {
                self.node_to_names.remove(&node_id);
            }
        }

        Ok(())
    }

    pub fn resolve(&self, name: &BeeName) -> Option<NodeId> {
        self.name_to_node.get(name).copied()
    }

    pub fn find_names_for_node(&self, node_id: NodeId) -> Vec<BeeName> {
        self.node_to_names
            .get(&node_id)
            .cloned()
            .unwrap_or_default()
    }

    pub fn list_names(&self) -> Vec<BeeName> {
        self.name_to_node.keys().cloned().collect()
    }

    pub fn clear(&mut self) {
        self.name_to_node.clear();
        self.node_to_names.clear();
    }
}
