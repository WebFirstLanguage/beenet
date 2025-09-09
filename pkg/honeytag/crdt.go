// Package honeytag implements CRDT Last-Writer-Wins register for conflict resolution
package honeytag

import (
	"strings"
)

// ConflictResolution implements CRDT Last-Writer-Wins register as specified in §12.3
// Conflict resolution rule: higher ver, else older ts, else lexicographically smaller owner BID

// CompareNameRecords compares two NameRecords and returns:
// -1 if a should be preferred over b
//
//	0 if they are equivalent
//	1 if b should be preferred over a
func CompareNameRecords(a, b *NameRecord) int {
	// Rule 1: Higher version wins
	if a.Ver > b.Ver {
		return -1
	}
	if b.Ver > a.Ver {
		return 1
	}

	// Rule 2: If versions are equal, older timestamp wins
	if a.TS < b.TS {
		return -1
	}
	if b.TS < a.TS {
		return 1
	}

	// Rule 3: If timestamps are equal, lexicographically smaller owner BID wins
	cmp := strings.Compare(a.Owner, b.Owner)
	if cmp < 0 {
		return -1
	}
	if cmp > 0 {
		return 1
	}

	// Records are equivalent
	return 0
}

// SelectWinningNameRecord selects the winning NameRecord from a slice using CRDT rules
func SelectWinningNameRecord(records []*NameRecord) *NameRecord {
	if len(records) == 0 {
		return nil
	}

	winner := records[0]
	for i := 1; i < len(records); i++ {
		if CompareNameRecords(records[i], winner) < 0 {
			winner = records[i]
		}
	}

	return winner
}

// CompareDelegationRecords compares two DelegationRecords using the same CRDT rules
func CompareDelegationRecords(a, b *DelegationRecord) int {
	// Rule 1: Higher version wins
	if a.Ver > b.Ver {
		return -1
	}
	if b.Ver > a.Ver {
		return 1
	}

	// Rule 2: If versions are equal, older timestamp wins
	if a.TS < b.TS {
		return -1
	}
	if b.TS < a.TS {
		return 1
	}

	// Rule 3: If timestamps are equal, lexicographically smaller owner BID wins
	cmp := strings.Compare(a.Owner, b.Owner)
	if cmp < 0 {
		return -1
	}
	if cmp > 0 {
		return 1
	}

	// Records are equivalent
	return 0
}

// SelectWinningDelegationRecord selects the winning DelegationRecord from a slice
func SelectWinningDelegationRecord(records []*DelegationRecord) *DelegationRecord {
	if len(records) == 0 {
		return nil
	}

	winner := records[0]
	for i := 1; i < len(records); i++ {
		if CompareDelegationRecords(records[i], winner) < 0 {
			winner = records[i]
		}
	}

	return winner
}

// ConflictSet represents a set of conflicting records for the same name
type ConflictSet struct {
	Name    string
	Records []*NameRecord
	Winner  *NameRecord
}

// NewConflictSet creates a new ConflictSet and determines the winner
func NewConflictSet(name string, records []*NameRecord) *ConflictSet {
	// Filter out expired records
	validRecords := make([]*NameRecord, 0, len(records))
	for _, record := range records {
		if !record.IsExpired() {
			validRecords = append(validRecords, record)
		}
	}

	return &ConflictSet{
		Name:    name,
		Records: validRecords,
		Winner:  SelectWinningNameRecord(validRecords),
	}
}

// HasConflicts returns true if there are multiple valid records for the same name
func (cs *ConflictSet) HasConflicts() bool {
	return len(cs.Records) > 1
}

// GetConflictingRecords returns all records except the winner
func (cs *ConflictSet) GetConflictingRecords() []*NameRecord {
	if cs.Winner == nil || len(cs.Records) <= 1 {
		return nil
	}

	conflicts := make([]*NameRecord, 0, len(cs.Records)-1)
	for _, record := range cs.Records {
		if record != cs.Winner {
			conflicts = append(conflicts, record)
		}
	}

	return conflicts
}

// DelegationConflictSet represents a set of conflicting delegation records
type DelegationConflictSet struct {
	Owner   string
	Records []*DelegationRecord
	Winner  *DelegationRecord
}

// NewDelegationConflictSet creates a new DelegationConflictSet and determines the winner
func NewDelegationConflictSet(owner string, records []*DelegationRecord) *DelegationConflictSet {
	// Filter out expired records
	validRecords := make([]*DelegationRecord, 0, len(records))
	for _, record := range records {
		if !record.IsExpired() {
			validRecords = append(validRecords, record)
		}
	}

	return &DelegationConflictSet{
		Owner:   owner,
		Records: validRecords,
		Winner:  SelectWinningDelegationRecord(validRecords),
	}
}

// HasConflicts returns true if there are multiple valid delegation records
func (dcs *DelegationConflictSet) HasConflicts() bool {
	return len(dcs.Records) > 1
}

// GetConflictingRecords returns all delegation records except the winner
func (dcs *DelegationConflictSet) GetConflictingRecords() []*DelegationRecord {
	if dcs.Winner == nil || len(dcs.Records) <= 1 {
		return nil
	}

	conflicts := make([]*DelegationRecord, 0, len(dcs.Records)-1)
	for _, record := range dcs.Records {
		if record != dcs.Winner {
			conflicts = append(conflicts, record)
		}
	}

	return conflicts
}

// FilterDelegationsByCapabilities filters delegation records by required capabilities
func FilterDelegationsByCapabilities(records []*DelegationRecord, requiredCaps []string) []*DelegationRecord {
	if len(requiredCaps) == 0 {
		return records
	}

	filtered := make([]*DelegationRecord, 0, len(records))
	for _, record := range records {
		if hasAllCapabilities(record.Caps, requiredCaps) {
			filtered = append(filtered, record)
		}
	}

	return filtered
}

// hasAllCapabilities checks if the record has all required capabilities
func hasAllCapabilities(recordCaps, requiredCaps []string) bool {
	for _, required := range requiredCaps {
		found := false
		for _, cap := range recordCaps {
			if cap == required {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
