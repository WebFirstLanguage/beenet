// Package agent implements supervisor pattern for agent lifecycle management
package agent

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// SupervisorConfig holds configuration for the supervisor
type SupervisorConfig struct {
	// MaxRetries is the maximum number of restart attempts
	MaxRetries int
	// RetryDelay is the delay between restart attempts
	RetryDelay time.Duration
	// HealthCheckInterval is how often to check agent health
	HealthCheckInterval time.Duration
}

// DefaultSupervisorConfig returns default supervisor configuration
func DefaultSupervisorConfig() SupervisorConfig {
	return SupervisorConfig{
		MaxRetries:          3,
		RetryDelay:          5 * time.Second,
		HealthCheckInterval: 10 * time.Second,
	}
}

// Supervisor manages an agent's lifecycle with restart capabilities
type Supervisor struct {
	mu     sync.RWMutex
	agent  *Agent
	config SupervisorConfig

	// Lifecycle management
	ctx        context.Context
	cancel     context.CancelFunc
	done       chan struct{}
	running    bool
	retryCount int
}

// NewSupervisor creates a new supervisor for the given agent
func NewSupervisor(agent *Agent) *Supervisor {
	return NewSupervisorWithConfig(agent, DefaultSupervisorConfig())
}

// NewSupervisorWithConfig creates a new supervisor with custom configuration
func NewSupervisorWithConfig(agent *Agent, config SupervisorConfig) *Supervisor {
	return &Supervisor{
		agent:  agent,
		config: config,
		done:   make(chan struct{}),
	}
}

// Start starts the supervisor
func (s *Supervisor) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return fmt.Errorf("supervisor is already running")
	}

	s.ctx, s.cancel = context.WithCancel(ctx)
	s.running = true
	s.retryCount = 0

	// Start the agent
	if err := s.agent.Start(s.ctx); err != nil {
		s.running = false
		return fmt.Errorf("failed to start agent: %w", err)
	}

	// Start supervisor loop
	go s.supervise()

	return nil
}

// Stop stops the supervisor and the managed agent
func (s *Supervisor) Stop(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return fmt.Errorf("supervisor is not running")
	}

	// Cancel supervisor context
	if s.cancel != nil {
		s.cancel()
	}

	// Stop the agent
	if err := s.agent.Stop(ctx); err != nil {
		return fmt.Errorf("failed to stop agent: %w", err)
	}

	// Wait for supervisor to finish
	select {
	case <-s.done:
		// Supervisor stopped gracefully
	case <-ctx.Done():
		// Timeout waiting for supervisor to stop
		return fmt.Errorf("timeout waiting for supervisor to stop")
	}

	s.running = false
	return nil
}

// IsRunning returns whether the supervisor is running
func (s *Supervisor) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// RetryCount returns the current retry count
func (s *Supervisor) RetryCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.retryCount
}

// supervise is the main supervisor loop
func (s *Supervisor) supervise() {
	defer close(s.done)

	ticker := time.NewTicker(s.config.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.checkAgentHealth()
		}
	}
}

// checkAgentHealth checks if the agent is healthy and restarts if needed
func (s *Supervisor) checkAgentHealth() {
	state := s.agent.State()

	// If agent is in error state or stopped unexpectedly, try to restart
	if state == StateError || (state == StateStopped && s.running) {
		s.mu.Lock()
		defer s.mu.Unlock()

		if s.retryCount >= s.config.MaxRetries {
			fmt.Printf("Supervisor: Maximum retries (%d) exceeded, giving up\n", s.config.MaxRetries)
			return
		}

		s.retryCount++
		fmt.Printf("Supervisor: Agent unhealthy (state: %s), attempting restart %d/%d\n",
			state, s.retryCount, s.config.MaxRetries)

		// Wait before retry
		time.Sleep(s.config.RetryDelay)

		// Try to restart the agent
		if err := s.agent.Start(s.ctx); err != nil {
			fmt.Printf("Supervisor: Failed to restart agent: %v\n", err)
		} else {
			fmt.Printf("Supervisor: Agent restarted successfully\n")
			// Reset retry count on successful restart
			s.retryCount = 0
		}
	}
}
