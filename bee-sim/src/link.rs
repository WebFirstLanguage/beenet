use std::time::Duration;

/// Link profile defining network characteristics
#[derive(Debug, Clone)]
pub struct LinkProfile {
    pub mtu: usize,
    pub latency: Duration,
    pub bandwidth: u64, // bits per second
    pub loss_rate: f32,
    pub duplication_rate: f32,
    pub duty_cycle: Option<DutyCycle>,
}

impl LinkProfile {
    pub fn perfect() -> Self {
        Self {
            mtu: 1500,
            latency: Duration::from_millis(1),
            bandwidth: 1_000_000_000, // 1 Gbps
            loss_rate: 0.0,
            duplication_rate: 0.0,
            duty_cycle: None,
        }
    }

    pub fn lossy() -> Self {
        Self {
            mtu: 1500,
            latency: Duration::from_millis(50),
            bandwidth: 10_000_000, // 10 Mbps
            loss_rate: 0.05,
            duplication_rate: 0.01,
            duty_cycle: None,
        }
    }

    pub fn constrained() -> Self {
        Self {
            mtu: 256,
            latency: Duration::from_millis(500),
            bandwidth: 9600, // 9600 bps
            loss_rate: 0.1,
            duplication_rate: 0.02,
            duty_cycle: Some(DutyCycle {
                on_duration: Duration::from_secs(10),
                off_duration: Duration::from_secs(50),
            }),
        }
    }
}

#[derive(Debug, Clone)]
pub struct DutyCycle {
    pub on_duration: Duration,
    pub off_duration: Duration,
}

impl Default for LinkProfile {
    fn default() -> Self {
        Self::perfect()
    }
}
