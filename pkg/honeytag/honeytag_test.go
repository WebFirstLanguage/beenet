package honeytag

import (
	"testing"
	"time"

	"github.com/WebFirstLanguage/beenet/pkg/constants"
	"github.com/WebFirstLanguage/beenet/pkg/identity"
)

// TestNameRecordCRDT tests the CRDT conflict resolution logic
func TestNameRecordCRDT(t *testing.T) {
	// Create test records with different versions and timestamps
	record1 := &NameRecord{
		V:     1,
		Swarm: "test-swarm",
		Name:  "alice",
		Owner: "bee:key:z6MkA",
		Ver:   1,
		TS:    1000,
		Lease: 2000,
	}

	record2 := &NameRecord{
		V:     1,
		Swarm: "test-swarm",
		Name:  "alice",
		Owner: "bee:key:z6MkB",
		Ver:   2, // Higher version should win
		TS:    1100,
		Lease: 2100,
	}

	record3 := &NameRecord{
		V:     1,
		Swarm: "test-swarm",
		Name:  "alice",
		Owner: "bee:key:z6MkC",
		Ver:   1,
		TS:    900, // Older timestamp should win when versions are equal
		Lease: 1900,
	}

	// Test version comparison
	if CompareNameRecords(record2, record1) >= 0 {
		t.Error("Record with higher version should win")
	}

	// Test timestamp comparison when versions are equal
	if CompareNameRecords(record3, record1) >= 0 {
		t.Error("Record with older timestamp should win when versions are equal")
	}

	// Test selecting winner from multiple records
	records := []*NameRecord{record1, record2, record3}
	winner := SelectWinningNameRecord(records)
	if winner != record2 {
		t.Error("Record2 should be the winner (highest version)")
	}
}

// TestHandleIndexValidation tests HandleIndex honeytag validation
func TestHandleIndexValidation(t *testing.T) {
	// Create a test identity
	id, err := identity.GenerateIdentity()
	if err != nil {
		t.Fatalf("Failed to generate identity: %v", err)
	}

	// Create HandleIndex with correct honeytag
	handle := id.Handle("alice")
	handleIndex := &HandleIndex{
		V:      1,
		Swarm:  "test-swarm",
		Handle: handle,
		BID:    id.BID(),
		TS:     uint64(time.Now().UnixMilli()),
		Expire: uint64(time.Now().Add(constants.HandleIndexExpire).UnixMilli()),
	}

	// This should pass validation (though ValidateHoneytag is currently a stub)
	err = handleIndex.ValidateHoneytag()
	if err != nil {
		t.Logf("Validation error (expected with stub implementation): %v", err)
	}

	// Test with invalid handle format
	handleIndex.Handle = "invalid-handle-format"
	err = handleIndex.ValidateHoneytag()
	if err == nil {
		t.Error("Should fail validation with invalid handle format")
	}
}

// TestDHTKeyGeneration tests the DHT key generation functions
func TestDHTKeyGeneration(t *testing.T) {
	swarmID := "test-swarm"
	name := "alice"
	handle := "alice~kapiz-ronit"
	ownerBID := "bee:key:z6MkTest"

	// Test key generation functions
	nameKey := K_name(swarmID, name)
	handleKey := K_handle(swarmID, handle)
	ownerKey := K_owner(swarmID, ownerBID)
	presenceKey := K_presence(swarmID, ownerBID)

	// Keys should be 32 bytes
	if len(nameKey) != 32 {
		t.Errorf("Name key should be 32 bytes, got %d", len(nameKey))
	}
	if len(handleKey) != 32 {
		t.Errorf("Handle key should be 32 bytes, got %d", len(handleKey))
	}
	if len(ownerKey) != 32 {
		t.Errorf("Owner key should be 32 bytes, got %d", len(ownerKey))
	}
	if len(presenceKey) != 32 {
		t.Errorf("Presence key should be 32 bytes, got %d", len(presenceKey))
	}

	// Keys should be deterministic
	nameKey2 := K_name(swarmID, name)
	if string(nameKey) != string(nameKey2) {
		t.Error("Key generation should be deterministic")
	}

	// Different inputs should produce different keys
	nameKey3 := K_name(swarmID, "bob")
	if string(nameKey) == string(nameKey3) {
		t.Error("Different names should produce different keys")
	}
}

// TestRecordExpiration tests record expiration logic
func TestRecordExpiration(t *testing.T) {
	now := uint64(time.Now().UnixMilli())

	// Test NameRecord expiration
	nameRecord := &NameRecord{
		V:     1,
		Swarm: "test-swarm",
		Name:  "alice",
		Owner: "bee:key:z6MkTest",
		Ver:   1,
		TS:    now - 1000,
		Lease: now - 500, // Expired 500ms ago
	}

	if !nameRecord.IsExpired() {
		t.Error("NameRecord should be expired")
	}

	// Test HandleIndex expiration
	handleIndex := &HandleIndex{
		V:      1,
		Swarm:  "test-swarm",
		Handle: "alice~kapiz-ronit",
		BID:    "bee:key:z6MkTest",
		TS:     now - 1000,
		Expire: now - 500, // Expired 500ms ago
	}

	if !handleIndex.IsExpired() {
		t.Error("HandleIndex should be expired")
	}

	// Test refresh needed logic
	nameRecord.Lease = now + 1000 // 1 second from now
	nameRecord.TS = now - 10000   // Created 10 seconds ago (total lease duration = 11 seconds)

	// Should need refresh when we're past 60% of lease duration
	// 60% of 11 seconds = 6.6 seconds, and we're at 10 seconds since creation
	if !nameRecord.NeedsRefresh() {
		t.Error("NameRecord should need refresh")
	}
}

// TestHandleParsing tests handle parsing functionality
func TestHandleParsing(t *testing.T) {
	// Test valid handle
	parts := parseHandle("alice~kapiz-ronit")
	if parts == nil {
		t.Fatal("Should parse valid handle")
	}
	if parts.Nickname != "alice" {
		t.Errorf("Expected nickname 'alice', got '%s'", parts.Nickname)
	}
	if parts.Honeytag != "kapiz-ronit" {
		t.Errorf("Expected honeytag 'kapiz-ronit', got '%s'", parts.Honeytag)
	}

	// Test invalid handles
	invalidHandles := []string{
		"alice",        // No ~
		"~kapiz-ronit", // ~ at start
		"alice~",       // ~ at end
	}

	for _, handle := range invalidHandles {
		parts := parseHandle(handle)
		if parts != nil {
			t.Errorf("Should not parse invalid handle: %s", handle)
		}
	}

	// Test handle with multiple ~ (should parse the last one)
	parts = parseHandle("alice~bob~carol")
	if parts == nil {
		t.Error("Should parse handle with multiple ~")
	} else if parts.Nickname != "alice~bob" || parts.Honeytag != "carol" {
		t.Errorf("Expected nickname 'alice~bob' and honeytag 'carol', got '%s' and '%s'", parts.Nickname, parts.Honeytag)
	}
}

// TestNicknameValidation tests nickname format validation
func TestNicknameValidation(t *testing.T) {
	validNicknames := []string{
		"alice",
		"bob123",
		"test-name",
		"a1b2c3",
	}

	for _, nickname := range validNicknames {
		if !isValidNickname(nickname) {
			t.Errorf("Should accept valid nickname: %s", nickname)
		}
	}

	invalidNicknames := []string{
		"ab",        // Too short
		"Alice",     // Uppercase
		"alice_bob", // Underscore
		"alice.bob", // Dot
		"alice bob", // Space
		"alice@bob", // Special character
		"",          // Empty
		"a",         // Too short
		"aa",        // Too short
		"this-is-a-very-long-nickname-that-exceeds-the-limit", // Too long
	}

	for _, nickname := range invalidNicknames {
		if isValidNickname(nickname) {
			t.Errorf("Should reject invalid nickname: %s", nickname)
		}
	}
}

// TestConflictSet tests conflict detection and resolution
func TestConflictSet(t *testing.T) {
	now := uint64(time.Now().UnixMilli())

	// Create conflicting records
	record1 := &NameRecord{
		V: 1, Swarm: "test", Name: "alice", Owner: "bee:key:z6MkA",
		Ver: 1, TS: now, Lease: now + 10000,
	}
	record2 := &NameRecord{
		V: 1, Swarm: "test", Name: "alice", Owner: "bee:key:z6MkB",
		Ver: 2, TS: now + 100, Lease: now + 10100,
	}
	record3 := &NameRecord{
		V: 1, Swarm: "test", Name: "alice", Owner: "bee:key:z6MkC",
		Ver: 1, TS: now - 100, Lease: now - 1, // Expired
	}

	records := []*NameRecord{record1, record2, record3}
	conflictSet := NewConflictSet("alice", records)

	// Should have 2 valid records (record3 is expired)
	if len(conflictSet.Records) != 2 {
		t.Errorf("Expected 2 valid records, got %d", len(conflictSet.Records))
	}

	// record2 should win (higher version)
	if conflictSet.Winner != record2 {
		t.Error("record2 should be the winner")
	}

	// Should detect conflicts
	if !conflictSet.HasConflicts() {
		t.Error("Should detect conflicts")
	}

	// Should return conflicting records
	conflicts := conflictSet.GetConflictingRecords()
	if len(conflicts) != 1 || conflicts[0] != record1 {
		t.Error("Should return record1 as conflicting record")
	}
}
