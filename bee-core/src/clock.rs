use std::time::Duration;

/// Virtual clock trait for deterministic time control
pub trait Clock: Send + Sync {
    /// Get current time as Duration since epoch
    fn now(&self) -> Duration;

    /// Advance the clock by specified duration
    fn advance(&mut self, delta: Duration);

    /// Tick the clock by one unit (implementation-defined)
    fn tick(&mut self);
}

/// Mock clock for testing with deterministic time control
#[derive(Debug, Clone)]
pub struct MockClock {
    current_time: Duration,
    tick_duration: Duration,
}

impl MockClock {
    pub fn new() -> Self {
        Self {
            current_time: Duration::ZERO,
            tick_duration: Duration::from_millis(1),
        }
    }

    pub fn with_tick_duration(tick_duration: Duration) -> Self {
        Self {
            current_time: Duration::ZERO,
            tick_duration,
        }
    }
}

impl Clock for MockClock {
    fn now(&self) -> Duration {
        self.current_time
    }

    fn advance(&mut self, delta: Duration) {
        self.current_time += delta;
    }

    fn tick(&mut self) {
        self.current_time += self.tick_duration;
    }
}

impl Default for MockClock {
    fn default() -> Self {
        Self::new()
    }
}
