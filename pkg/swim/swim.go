// Package swim implements the SWIM (Scalable Weakly-consistent Infection-style Process Group Membership) protocol
package swim

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/WebFirstLanguage/beenet/pkg/constants"
	"github.com/WebFirstLanguage/beenet/pkg/identity"
	"github.com/WebFirstLanguage/beenet/pkg/wire"
)

// NetworkInterface defines the interface for network operations
type NetworkInterface interface {
	SendMessage(ctx context.Context, target *Member, frame *wire.BaseFrame) error
	BroadcastMessage(ctx context.Context, frame *wire.BaseFrame) error
}

// Config holds SWIM configuration
type Config struct {
	Identity         *identity.Identity // Local node identity
	SwarmID          string             // Swarm identifier
	Network          NetworkInterface   // Network interface for sending messages
	BindAddr         string             // Local bind address
	ProbeInterval    time.Duration      // Interval between probes (default: 5s)
	PingTimeout      time.Duration      // Direct ping timeout (default: 1s)
	IndirectTimeout  time.Duration      // Indirect ping timeout (default: 3s)
	SuspicionTimeout time.Duration      // Time to remain in suspect state (default: 10s)
}

// SWIM represents a SWIM protocol instance
type SWIM struct {
	mu sync.RWMutex

	// Configuration
	identity         *identity.Identity
	swarmID          string
	network          NetworkInterface
	bindAddr         string
	probeInterval    time.Duration
	pingTimeout      time.Duration
	indirectTimeout  time.Duration
	suspicionTimeout time.Duration

	// Local member information
	localMember *Member
	incarnation uint64 // Our current incarnation number
	sequenceNum uint64 // Sequence number for messages

	// Membership list
	members map[string]*Member // BID -> Member

	// Failure detection state
	probeTarget   *Member                       // Current member being probed
	pendingPings  map[uint64]*Member            // seqNo -> target member for pending pings
	indirectPings map[uint64]*indirectPingState // seqNo -> indirect ping state

	// Lifecycle
	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}
}

// indirectPingState tracks the state of an indirect ping operation
type indirectPingState struct {
	target    *Member
	requestor string
	startTime time.Time
	timeout   time.Duration
}

// New creates a new SWIM instance
func New(config *Config) (*SWIM, error) {
	if config.Identity == nil {
		return nil, fmt.Errorf("identity is required")
	}

	if config.SwarmID == "" {
		return nil, fmt.Errorf("swarm ID is required")
	}

	if config.Network == nil {
		return nil, fmt.Errorf("network interface is required")
	}

	// Set default timeouts if not provided
	probeInterval := config.ProbeInterval
	if probeInterval == 0 {
		probeInterval = constants.SWIMProbeInterval
	}

	pingTimeout := config.PingTimeout
	if pingTimeout == 0 {
		pingTimeout = constants.SWIMPingTimeout
	}

	indirectTimeout := config.IndirectTimeout
	if indirectTimeout == 0 {
		indirectTimeout = constants.SWIMIndirectTimeout
	}

	suspicionTimeout := config.SuspicionTimeout
	if suspicionTimeout == 0 {
		suspicionTimeout = constants.SWIMSuspicionTime
	}

	// Create local member
	localAddrs := []string{config.BindAddr}
	localMember := NewMember(config.Identity.BID(), localAddrs)

	swim := &SWIM{
		identity:         config.Identity,
		swarmID:          config.SwarmID,
		network:          config.Network,
		bindAddr:         config.BindAddr,
		probeInterval:    probeInterval,
		pingTimeout:      pingTimeout,
		indirectTimeout:  indirectTimeout,
		suspicionTimeout: suspicionTimeout,
		localMember:      localMember,
		incarnation:      0,
		sequenceNum:      0,
		members:          make(map[string]*Member),
		pendingPings:     make(map[uint64]*Member),
		indirectPings:    make(map[uint64]*indirectPingState),
		done:             make(chan struct{}),
	}

	return swim, nil
}

// Start starts the SWIM protocol
func (s *SWIM) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.ctx != nil {
		return fmt.Errorf("SWIM is already running")
	}

	s.ctx, s.cancel = context.WithCancel(ctx)

	// Start the probe loop
	go s.probeLoop()

	return nil
}

// Stop stops the SWIM protocol
func (s *SWIM) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}

	return nil
}

// AddMember adds a new member to the membership list
func (s *SWIM) AddMember(bid string, addrs []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if bid == s.identity.BID() {
		return fmt.Errorf("cannot add self as member")
	}

	if _, exists := s.members[bid]; exists {
		// Update addresses if member already exists
		s.members[bid].UpdateAddresses(addrs)
		return nil
	}

	member := NewMember(bid, addrs)
	s.members[bid] = member

	return nil
}

// GetMember returns a member by BID
func (s *SWIM) GetMember(bid string) *Member {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.members[bid]
}

// GetMembers returns a copy of all members
func (s *SWIM) GetMembers() []*Member {
	s.mu.RLock()
	defer s.mu.RUnlock()

	members := make([]*Member, 0, len(s.members))
	for _, member := range s.members {
		members = append(members, member)
	}

	return members
}

// PingMember sends a direct ping to a member
func (s *SWIM) PingMember(ctx context.Context, target *Member) error {
	seqNo := s.getNextSequence()

	// Create SWIM_PING message
	pingFrame := wire.NewSWIMPingFrame(s.identity.BID(), s.getNextSequence(), target.BID, seqNo)

	// Sign the frame
	if err := pingFrame.Sign(s.identity.SigningPrivateKey); err != nil {
		return fmt.Errorf("failed to sign ping frame: %w", err)
	}

	// Store pending ping
	s.mu.Lock()
	s.pendingPings[seqNo] = target
	s.mu.Unlock()

	// Update last ping time
	target.UpdateLastPing()

	// Send the ping
	return s.network.SendMessage(ctx, target, pingFrame)
}

// HandleMessage handles incoming SWIM protocol messages
func (s *SWIM) HandleMessage(ctx context.Context, frame *wire.BaseFrame) error {
	switch frame.Kind {
	case constants.KindSWIMPing:
		return s.handlePing(ctx, frame)
	case constants.KindSWIMAck:
		return s.handleAck(ctx, frame)
	case constants.KindSWIMNack:
		return s.handleNack(ctx, frame)
	case constants.KindSWIMPingReq:
		return s.handlePingReq(ctx, frame)
	case constants.KindSWIMPingResp:
		return s.handlePingResp(ctx, frame)
	case constants.KindSWIMSuspect:
		return s.handleSuspect(ctx, frame)
	case constants.KindSWIMAlive:
		return s.handleAlive(ctx, frame)
	case constants.KindSWIMConfirm:
		return s.handleConfirm(ctx, frame)
	case constants.KindSWIMLeave:
		return s.handleLeave(ctx, frame)
	default:
		return fmt.Errorf("unsupported SWIM message kind: %d", frame.Kind)
	}
}

// getNextSequence returns the next sequence number
func (s *SWIM) getNextSequence() uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sequenceNum++
	return s.sequenceNum
}

// probeLoop runs the periodic probing of members
func (s *SWIM) probeLoop() {
	ticker := time.NewTicker(s.probeInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.probeRandomMember()
		}
	}
}

// probeRandomMember selects a random member and probes it
func (s *SWIM) probeRandomMember() {
	s.mu.RLock()
	members := make([]*Member, 0, len(s.members))
	for _, member := range s.members {
		if member.IsAlive() {
			members = append(members, member)
		}
	}
	s.mu.RUnlock()

	if len(members) == 0 {
		return
	}

	// Select random member
	n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(members))))
	target := members[n.Int64()]

	// Probe the member
	ctx, cancel := context.WithTimeout(s.ctx, s.pingTimeout)
	defer cancel()

	if err := s.PingMember(ctx, target); err != nil {
		// If direct ping fails, try indirect ping
		s.indirectPing(target)
	}
}

// indirectPing attempts to ping a member through intermediaries
func (s *SWIM) indirectPing(target *Member) {
	// Implementation will be added in the next iteration
	// For now, just mark as suspect
	target.SetState(StateSuspect, target.Incarnation)
}

// handlePing handles incoming SWIM_PING messages
func (s *SWIM) handlePing(ctx context.Context, frame *wire.BaseFrame) error {
	body, ok := frame.Body.(*wire.SWIMPingBody)
	if !ok {
		return fmt.Errorf("invalid SWIM_PING body")
	}

	// Send ACK response
	ackFrame := wire.NewSWIMAckFrame(s.identity.BID(), s.getNextSequence(), body.SeqNo)
	if err := ackFrame.Sign(s.identity.SigningPrivateKey); err != nil {
		return fmt.Errorf("failed to sign ack frame: %w", err)
	}

	// Find the sender member to send response
	sender := s.GetMember(frame.From)
	if sender == nil {
		// If we don't know the sender, we can't send a response
		// In a full implementation, we might add them to our member list
		return fmt.Errorf("unknown sender: %s", frame.From)
	}

	return s.network.SendMessage(ctx, sender, ackFrame)
}

// handleAck handles incoming SWIM_ACK messages
func (s *SWIM) handleAck(ctx context.Context, frame *wire.BaseFrame) error {
	body, ok := frame.Body.(*wire.SWIMAckBody)
	if !ok {
		return fmt.Errorf("invalid SWIM_ACK body")
	}

	// Find the pending ping
	s.mu.Lock()
	target, exists := s.pendingPings[body.SeqNo]
	if exists {
		delete(s.pendingPings, body.SeqNo)
	}
	s.mu.Unlock()

	if !exists {
		// ACK for unknown ping, ignore
		return nil
	}

	// Update member as alive
	target.UpdateLastSeen()
	if target.IsSuspect() {
		target.SetState(StateAlive, target.Incarnation+1)
	}

	return nil
}

// handleNack handles incoming SWIM_NACK messages
func (s *SWIM) handleNack(ctx context.Context, frame *wire.BaseFrame) error {
	body, ok := frame.Body.(*wire.SWIMNackBody)
	if !ok {
		return fmt.Errorf("invalid SWIM_NACK body")
	}

	// Find the pending ping
	s.mu.Lock()
	target, exists := s.pendingPings[body.SeqNo]
	if exists {
		delete(s.pendingPings, body.SeqNo)
	}
	s.mu.Unlock()

	if !exists {
		// NACK for unknown ping, ignore
		return nil
	}

	// Mark member as suspect
	target.SetState(StateSuspect, target.Incarnation)

	return nil
}

// handlePingReq handles incoming SWIM_PING_REQ messages (indirect ping requests)
func (s *SWIM) handlePingReq(ctx context.Context, frame *wire.BaseFrame) error {
	// Implementation will be added in the next iteration
	return nil
}

// handlePingResp handles incoming SWIM_PING_RESP messages (indirect ping responses)
func (s *SWIM) handlePingResp(ctx context.Context, frame *wire.BaseFrame) error {
	// Implementation will be added in the next iteration
	return nil
}

// handleSuspect handles incoming SWIM_SUSPECT messages
func (s *SWIM) handleSuspect(ctx context.Context, frame *wire.BaseFrame) error {
	// Implementation will be added in the next iteration
	return nil
}

// handleAlive handles incoming SWIM_ALIVE messages
func (s *SWIM) handleAlive(ctx context.Context, frame *wire.BaseFrame) error {
	// Implementation will be added in the next iteration
	return nil
}

// handleConfirm handles incoming SWIM_CONFIRM messages
func (s *SWIM) handleConfirm(ctx context.Context, frame *wire.BaseFrame) error {
	// Implementation will be added in the next iteration
	return nil
}

// handleLeave handles incoming SWIM_LEAVE messages
func (s *SWIM) handleLeave(ctx context.Context, frame *wire.BaseFrame) error {
	// Implementation will be added in the next iteration
	return nil
}
