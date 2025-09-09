// Package noiseik implements replay protection using sliding window mechanism
package noiseik

import (
	"sync"
)

// ReplayWindow implements a sliding window for replay protection
// It uses a bitmap to track received sequence numbers within a window
type ReplayWindow struct {
	mu           sync.RWMutex
	windowSize   uint64   // Size of the replay window
	lastSequence uint64   // Highest sequence number seen
	bitmap       []uint64 // Bitmap to track received sequences
}

// NewReplayWindow creates a new replay protection window
func NewReplayWindow(windowSize uint64) *ReplayWindow {
	if windowSize == 0 {
		windowSize = 64 // Default window size
	}
	
	// Calculate number of uint64s needed for the bitmap
	bitmapSize := (windowSize + 63) / 64
	
	return &ReplayWindow{
		windowSize: windowSize,
		bitmap:     make([]uint64, bitmapSize),
	}
}

// AcceptSequence checks if a sequence number should be accepted
// Returns true if the sequence is valid and not a replay
func (rw *ReplayWindow) AcceptSequence(sequence uint64) bool {
	rw.mu.Lock()
	defer rw.mu.Unlock()
	
	// Reject sequence number 0 (invalid)
	if sequence == 0 {
		return false
	}
	
	// If this is the first sequence or a new highest sequence
	if sequence > rw.lastSequence {
		// Slide the window if necessary
		rw.slideWindow(sequence)
		rw.lastSequence = sequence
		rw.setBit(sequence)
		return true
	}
	
	// Check if sequence is within the current window
	if rw.lastSequence-sequence >= rw.windowSize {
		// Sequence is too old, outside the window
		return false
	}
	
	// Check if we've already seen this sequence
	if rw.getBit(sequence) {
		// Duplicate sequence - replay attack
		return false
	}
	
	// Accept the sequence and mark it as seen
	rw.setBit(sequence)
	return true
}

// slideWindow slides the replay window to accommodate a new highest sequence
func (rw *ReplayWindow) slideWindow(newSequence uint64) {
	if newSequence <= rw.lastSequence {
		return
	}
	
	shift := newSequence - rw.lastSequence
	
	// If the shift is larger than the window, clear everything
	if shift >= rw.windowSize {
		for i := range rw.bitmap {
			rw.bitmap[i] = 0
		}
		return
	}
	
	// Shift the bitmap
	rw.shiftBitmap(shift)
}

// shiftBitmap shifts the bitmap left by the specified number of bits
func (rw *ReplayWindow) shiftBitmap(shift uint64) {
	if shift == 0 {
		return
	}
	
	wordShift := shift / 64
	bitShift := shift % 64
	
	// Shift by whole words first
	if wordShift > 0 {
		for i := len(rw.bitmap) - 1; i >= int(wordShift); i-- {
			rw.bitmap[i] = rw.bitmap[i-int(wordShift)]
		}
		for i := 0; i < int(wordShift); i++ {
			rw.bitmap[i] = 0
		}
	}
	
	// Shift by remaining bits
	if bitShift > 0 {
		carry := uint64(0)
		for i := 0; i < len(rw.bitmap); i++ {
			newCarry := rw.bitmap[i] >> (64 - bitShift)
			rw.bitmap[i] = (rw.bitmap[i] << bitShift) | carry
			carry = newCarry
		}
	}
}

// setBit sets the bit for a given sequence number
func (rw *ReplayWindow) setBit(sequence uint64) {
	if sequence > rw.lastSequence {
		return
	}
	
	offset := rw.lastSequence - sequence
	if offset >= rw.windowSize {
		return
	}
	
	wordIndex := offset / 64
	bitIndex := offset % 64
	
	if int(wordIndex) < len(rw.bitmap) {
		rw.bitmap[wordIndex] |= (1 << bitIndex)
	}
}

// getBit gets the bit for a given sequence number
func (rw *ReplayWindow) getBit(sequence uint64) bool {
	if sequence > rw.lastSequence {
		return false
	}
	
	offset := rw.lastSequence - sequence
	if offset >= rw.windowSize {
		return false
	}
	
	wordIndex := offset / 64
	bitIndex := offset % 64
	
	if int(wordIndex) < len(rw.bitmap) {
		return (rw.bitmap[wordIndex] & (1 << bitIndex)) != 0
	}
	
	return false
}

// SequenceTracker manages sequence numbers for both sending and receiving
type SequenceTracker struct {
	mu           sync.Mutex
	sendSequence uint64       // Next sequence number to send
	recvWindow   *ReplayWindow // Replay protection for received messages
}

// NewSequenceTracker creates a new sequence tracker
func NewSequenceTracker() *SequenceTracker {
	return &SequenceTracker{
		sendSequence: 0,
		recvWindow:   NewReplayWindow(64), // 64-bit window
	}
}

// NextSendSequence returns the next sequence number for sending
func (st *SequenceTracker) NextSendSequence() uint64 {
	st.mu.Lock()
	defer st.mu.Unlock()
	
	st.sendSequence++
	return st.sendSequence
}

// ValidateReceiveSequence validates a received sequence number
// Returns true if the sequence is valid and not a replay
func (st *SequenceTracker) ValidateReceiveSequence(sequence uint64) bool {
	return st.recvWindow.AcceptSequence(sequence)
}

// GetSendSequence returns the current send sequence number (for testing)
func (st *SequenceTracker) GetSendSequence() uint64 {
	st.mu.Lock()
	defer st.mu.Unlock()
	return st.sendSequence
}

// GetLastReceivedSequence returns the last received sequence number (for testing)
func (st *SequenceTracker) GetLastReceivedSequence() uint64 {
	st.recvWindow.mu.RLock()
	defer st.recvWindow.mu.RUnlock()
	return st.recvWindow.lastSequence
}
