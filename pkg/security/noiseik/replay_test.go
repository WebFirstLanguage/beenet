package noiseik

import (
	"testing"
)

func TestReplayWindow_NewWindow(t *testing.T) {
	window := NewReplayWindow(64)
	
	if window.windowSize != 64 {
		t.Errorf("Expected window size 64, got %d", window.windowSize)
	}
	
	if window.lastSequence != 0 {
		t.Errorf("Expected initial sequence 0, got %d", window.lastSequence)
	}
	
	if len(window.bitmap) != 1 {
		t.Errorf("Expected bitmap length 1, got %d", len(window.bitmap))
	}
}

func TestReplayWindow_AcceptSequence(t *testing.T) {
	window := NewReplayWindow(64)
	
	// Test accepting first sequence
	if !window.AcceptSequence(1) {
		t.Error("Should accept first sequence number 1")
	}
	
	// Test accepting increasing sequences
	if !window.AcceptSequence(2) {
		t.Error("Should accept sequence number 2")
	}
	
	if !window.AcceptSequence(5) {
		t.Error("Should accept sequence number 5")
	}
	
	// Test rejecting duplicate sequence
	if window.AcceptSequence(5) {
		t.Error("Should reject duplicate sequence number 5")
	}
	
	if window.AcceptSequence(2) {
		t.Error("Should reject duplicate sequence number 2")
	}
}

func TestReplayWindow_OutOfOrderAcceptance(t *testing.T) {
	window := NewReplayWindow(64)
	
	// Accept sequence 10
	if !window.AcceptSequence(10) {
		t.Error("Should accept sequence number 10")
	}
	
	// Accept sequence 5 (within window)
	if !window.AcceptSequence(5) {
		t.Error("Should accept sequence number 5 within window")
	}
	
	// Accept sequence 8 (within window)
	if !window.AcceptSequence(8) {
		t.Error("Should accept sequence number 8 within window")
	}
	
	// Reject duplicate sequence 8
	if window.AcceptSequence(8) {
		t.Error("Should reject duplicate sequence number 8")
	}
}

func TestReplayWindow_WindowSliding(t *testing.T) {
	window := NewReplayWindow(4) // Small window for testing
	
	// Fill the window
	window.AcceptSequence(1)
	window.AcceptSequence(2)
	window.AcceptSequence(3)
	window.AcceptSequence(4)
	
	// Accept sequence 8 - should slide the window
	if !window.AcceptSequence(8) {
		t.Error("Should accept sequence number 8 and slide window")
	}
	
	// Sequence 1 should now be outside the window and rejected
	if window.AcceptSequence(1) {
		t.Error("Should reject sequence number 1 (outside window)")
	}
	
	// Sequence 5 should be within the new window
	if !window.AcceptSequence(5) {
		t.Error("Should accept sequence number 5 within new window")
	}
}

func TestReplayWindow_LargeGaps(t *testing.T) {
	window := NewReplayWindow(64)
	
	// Accept sequence 1
	window.AcceptSequence(1)
	
	// Accept sequence 1000 (large gap)
	if !window.AcceptSequence(1000) {
		t.Error("Should accept sequence number 1000")
	}
	
	// Sequence 1 should now be outside the window
	if window.AcceptSequence(1) {
		t.Error("Should reject sequence number 1 (outside window after large gap)")
	}
	
	// Sequence 950 should be within the window
	if !window.AcceptSequence(950) {
		t.Error("Should accept sequence number 950 within window")
	}
}

func TestReplayWindow_ZeroSequence(t *testing.T) {
	window := NewReplayWindow(64)
	
	// Sequence 0 should be rejected (invalid)
	if window.AcceptSequence(0) {
		t.Error("Should reject sequence number 0")
	}
}

func TestSequenceTracker_NewTracker(t *testing.T) {
	tracker := NewSequenceTracker()
	
	if tracker.sendSequence != 0 {
		t.Errorf("Expected initial send sequence 0, got %d", tracker.sendSequence)
	}
	
	if tracker.recvWindow == nil {
		t.Error("Expected receive window to be initialized")
	}
}

func TestSequenceTracker_NextSendSequence(t *testing.T) {
	tracker := NewSequenceTracker()
	
	// Get first sequence number
	seq1 := tracker.NextSendSequence()
	if seq1 != 1 {
		t.Errorf("Expected first sequence number 1, got %d", seq1)
	}
	
	// Get second sequence number
	seq2 := tracker.NextSendSequence()
	if seq2 != 2 {
		t.Errorf("Expected second sequence number 2, got %d", seq2)
	}
	
	// Verify sequences are increasing
	if seq2 <= seq1 {
		t.Error("Sequence numbers should be strictly increasing")
	}
}

func TestSequenceTracker_ValidateReceiveSequence(t *testing.T) {
	tracker := NewSequenceTracker()
	
	// Accept first sequence
	if !tracker.ValidateReceiveSequence(1) {
		t.Error("Should accept first receive sequence 1")
	}
	
	// Accept second sequence
	if !tracker.ValidateReceiveSequence(2) {
		t.Error("Should accept receive sequence 2")
	}
	
	// Reject duplicate sequence
	if tracker.ValidateReceiveSequence(2) {
		t.Error("Should reject duplicate receive sequence 2")
	}
	
	// Accept out-of-order sequence within window
	if !tracker.ValidateReceiveSequence(5) {
		t.Error("Should accept out-of-order receive sequence 5")
	}
	
	if !tracker.ValidateReceiveSequence(3) {
		t.Error("Should accept out-of-order receive sequence 3")
	}
}

func TestSequenceTracker_ReplayAttack(t *testing.T) {
	tracker := NewSequenceTracker()
	
	// Simulate normal message flow
	sequences := []uint64{1, 2, 3, 5, 4, 6, 8, 7, 9, 10}
	
	for _, seq := range sequences {
		if !tracker.ValidateReceiveSequence(seq) {
			t.Errorf("Should accept sequence %d in normal flow", seq)
		}
	}
	
	// Try to replay some sequences - should be rejected
	replaySequences := []uint64{1, 3, 5, 7, 9}
	
	for _, seq := range replaySequences {
		if tracker.ValidateReceiveSequence(seq) {
			t.Errorf("Should reject replayed sequence %d", seq)
		}
	}
}

func TestSequenceTracker_WindowOverflow(t *testing.T) {
	tracker := NewSequenceTracker()
	
	// Accept a sequence far in the future
	if !tracker.ValidateReceiveSequence(1000) {
		t.Error("Should accept sequence 1000")
	}
	
	// Try to accept sequences that are now outside the window
	oldSequences := []uint64{1, 2, 3, 935} // 935 is at the edge of a 64-bit window
	
	for _, seq := range oldSequences {
		if tracker.ValidateReceiveSequence(seq) {
			t.Errorf("Should reject old sequence %d (outside window)", seq)
		}
	}
	
	// Accept sequences within the new window
	newSequences := []uint64{950, 980, 999}
	
	for _, seq := range newSequences {
		if !tracker.ValidateReceiveSequence(seq) {
			t.Errorf("Should accept sequence %d within new window", seq)
		}
	}
}
