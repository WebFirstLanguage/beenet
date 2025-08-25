use bee_core::identity::NodeId;

/// Network topology builder
pub struct TopologyBuilder;

impl TopologyBuilder {
    /// Create a line topology: n0 - n1 - n2 - ... - nN
    pub fn line(nodes: Vec<NodeId>) -> Vec<(NodeId, NodeId)> {
        let mut edges = Vec::new();
        for i in 0..nodes.len().saturating_sub(1) {
            edges.push((nodes[i], nodes[i + 1]));
        }
        edges
    }

    /// Create a ring topology: n0 - n1 - n2 - ... - nN - n0
    pub fn ring(nodes: Vec<NodeId>) -> Vec<(NodeId, NodeId)> {
        let mut edges = Self::line(nodes.clone());
        if nodes.len() > 2 {
            edges.push((nodes[nodes.len() - 1], nodes[0]));
        }
        edges
    }

    /// Create a grid topology
    pub fn grid(nodes: Vec<NodeId>, width: usize) -> Vec<(NodeId, NodeId)> {
        let mut edges = Vec::new();
        let height = nodes.len().div_ceil(width);

        for y in 0..height {
            for x in 0..width {
                let idx = y * width + x;
                if idx >= nodes.len() {
                    break;
                }

                // Connect to right neighbor
                if x + 1 < width && idx + 1 < nodes.len() {
                    edges.push((nodes[idx], nodes[idx + 1]));
                }

                // Connect to bottom neighbor
                if y + 1 < height && idx + width < nodes.len() {
                    edges.push((nodes[idx], nodes[idx + width]));
                }
            }
        }

        edges
    }

    /// Create a fully connected mesh
    pub fn full_mesh(nodes: Vec<NodeId>) -> Vec<(NodeId, NodeId)> {
        let mut edges = Vec::new();
        for i in 0..nodes.len() {
            for j in i + 1..nodes.len() {
                edges.push((nodes[i], nodes[j]));
            }
        }
        edges
    }
}
