// Package honeytag implements the deterministic resolution algorithm as specified in §12.5
package honeytag

import (
	"context"
	"fmt"
	"strings"
	"unicode"

	"github.com/WebFirstLanguage/beenet/internal/dht"
	"github.com/WebFirstLanguage/beenet/pkg/codec/cborcanon"
	"github.com/WebFirstLanguage/beenet/pkg/identity"
	"golang.org/x/text/unicode/norm"
)

// Resolver implements the deterministic resolution algorithm from §12.5
type Resolver struct {
	dht     *dht.DHT
	swarmID string
	cache   *ResolverCache
}

// NewResolver creates a new honeytag resolver
func NewResolver(dht *dht.DHT, swarmID string) *Resolver {
	return &Resolver{
		dht:     dht,
		swarmID: swarmID,
		cache:   NewResolverCache(),
	}
}

// ResolveResult represents the result of a resolution operation
type ResolveResult struct {
	Kind   string       // "bid"|"handle"|"bare"
	Owner  string       // Owner BID if known
	Device string       // Device BID (may be same as owner)
	Handle string       // Handle if applicable
	Addrs  []string     // Multiaddresses if available
	Proof  ResolveProof // Cryptographic proofs
}

// ResolveProof contains cryptographic proofs for resolution
type ResolveProof struct {
	Name        *NameRecord         // NameRecord if applicable
	HandleIndex *HandleIndex        // HandleIndex if applicable
	Presence    *dht.PresenceRecord // PresenceRecord if applicable
	Delegation  *DelegationRecord   // DelegationRecord if applicable
	Conflicts   []*NameRecord       // Conflicting records if any
}

// Resolve implements the deterministic resolution algorithm from §12.5
func (r *Resolver) Resolve(ctx context.Context, query string, preferredCaps []string) (*ResolveResult, error) {
	// Step 1: Normalize query (trim, NFKC, lowercase nickname)
	normalized := r.normalize(query)

	// Step 2: Check if query is a BID
	if r.isBID(normalized) {
		return r.resolveBID(ctx, normalized)
	}

	// Step 3: Check if query is a handle
	if r.isHandle(normalized) {
		return r.resolveHandle(ctx, normalized)
	}

	// Step 4: Treat query as bare name
	return r.resolveBare(ctx, normalized, preferredCaps)
}

// normalize implements query normalization as specified
func (r *Resolver) normalize(query string) string {
	// Trim whitespace
	trimmed := strings.TrimSpace(query)

	// Apply NFKC normalization
	normalized := norm.NFKC.String(trimmed)

	// For bare names, lowercase the nickname part
	if !r.isBID(normalized) && !r.isHandle(normalized) {
		return strings.ToLower(normalized)
	}

	return normalized
}

// isBID checks if the query is a BID (starts with "bee:key:")
func (r *Resolver) isBID(query string) bool {
	return strings.HasPrefix(query, "bee:key:")
}

// isHandle checks if the query contains a ~ character (handle format)
func (r *Resolver) isHandle(query string) bool {
	return strings.Contains(query, "~")
}

// resolveBID resolves a BID query
func (r *Resolver) resolveBID(ctx context.Context, bid string) (*ResolveResult, error) {
	// Fetch PresenceRecord at K_presence
	key := K_presence(r.swarmID, bid)
	presenceData, err := r.dht.Get(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("failed to get presence record: %w", err)
	}

	var presence *dht.PresenceRecord
	if presenceData != nil {
		presence = &dht.PresenceRecord{}
		if err := cborcanon.Unmarshal(presenceData, presence); err != nil {
			return nil, fmt.Errorf("failed to unmarshal presence record: %w", err)
		}

		// Security guard: validate honeytag matches
		if err := r.validatePresenceHoneytag(presence); err != nil {
			return nil, fmt.Errorf("presence validation failed: %w", err)
		}
	}

	return &ResolveResult{
		Kind:   "bid",
		Owner:  bid,
		Device: bid,
		Handle: r.synthesizeHandle(bid, presence),
		Addrs:  r.extractAddresses(presence),
		Proof: ResolveProof{
			Presence: presence,
		},
	}, nil
}

// resolveHandle resolves a handle query
func (r *Resolver) resolveHandle(ctx context.Context, handle string) (*ResolveResult, error) {
	// GET K_handle
	key := K_handle(r.swarmID, handle)
	handleData, err := r.dht.Get(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("failed to get handle index: %w", err)
	}

	if handleData == nil {
		return nil, fmt.Errorf("handle not found: %s", handle)
	}

	var handleIndex HandleIndex
	if err := cborcanon.Unmarshal(handleData, &handleIndex); err != nil {
		return nil, fmt.Errorf("failed to unmarshal handle index: %w", err)
	}

	// Verify honeytag(bid) equals suffix in handle
	if err := handleIndex.ValidateHoneytag(); err != nil {
		return nil, fmt.Errorf("honeytag validation failed: %w", err)
	}

	// Fetch PresenceRecord for that BID
	presenceKey := K_presence(r.swarmID, handleIndex.BID)
	presenceData, err := r.dht.Get(ctx, presenceKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get presence record: %w", err)
	}

	var presence *dht.PresenceRecord
	if presenceData != nil {
		presence = &dht.PresenceRecord{}
		if err := cborcanon.Unmarshal(presenceData, presence); err != nil {
			return nil, fmt.Errorf("failed to unmarshal presence record: %w", err)
		}

		// Security guard: validate honeytag matches
		if err := r.validatePresenceHoneytag(presence); err != nil {
			return nil, fmt.Errorf("presence validation failed: %w", err)
		}
	}

	return &ResolveResult{
		Kind:   "handle",
		Owner:  handleIndex.BID,
		Device: handleIndex.BID,
		Handle: handle,
		Addrs:  r.extractAddresses(presence),
		Proof: ResolveProof{
			HandleIndex: &handleIndex,
			Presence:    presence,
		},
	}, nil
}

// resolveBare resolves a bare name query
func (r *Resolver) resolveBare(ctx context.Context, name string, preferredCaps []string) (*ResolveResult, error) {
	// GET K_name
	key := K_name(r.swarmID, name)
	nameData, err := r.dht.Get(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("failed to get name record: %w", err)
	}

	if nameData == nil {
		return nil, fmt.Errorf("name not found: %s", name)
	}

	// For now, assume single record. In full implementation, would handle multiple records
	var nameRecord NameRecord
	if err := cborcanon.Unmarshal(nameData, &nameRecord); err != nil {
		return nil, fmt.Errorf("failed to unmarshal name record: %w", err)
	}

	// Check if lease is valid
	if nameRecord.IsExpired() {
		return nil, fmt.Errorf("name lease expired: %s", name)
	}

	owner := nameRecord.Owner

	// Select a device (simplified - in full implementation would check delegations)
	device := owner

	// Fetch PresenceRecord for chosen device
	presenceKey := K_presence(r.swarmID, device)
	presenceData, err := r.dht.Get(ctx, presenceKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get presence record: %w", err)
	}

	var presence *dht.PresenceRecord
	if presenceData != nil {
		presence = &dht.PresenceRecord{}
		if err := cborcanon.Unmarshal(presenceData, presence); err != nil {
			return nil, fmt.Errorf("failed to unmarshal presence record: %w", err)
		}

		// Security guard: validate honeytag matches
		if err := r.validatePresenceHoneytag(presence); err != nil {
			return nil, fmt.Errorf("presence validation failed: %w", err)
		}
	}

	return &ResolveResult{
		Kind:   "bare",
		Owner:  owner,
		Device: device,
		Handle: r.synthesizeHandle(device, presence),
		Addrs:  r.extractAddresses(presence),
		Proof: ResolveProof{
			Name:     &nameRecord,
			Presence: presence,
		},
	}, nil
}

// validatePresenceHoneytag validates that the presence record's handle matches the BID's honeytag
func (r *Resolver) validatePresenceHoneytag(presence *dht.PresenceRecord) error {
	if presence == nil {
		return nil // No presence record to validate
	}

	// Extract honeytag from handle
	handleParts := parseHandle(presence.Handle)
	if handleParts == nil {
		return fmt.Errorf("invalid handle format in presence: %s", presence.Handle)
	}

	// Validate honeytag matches BID
	return identity.ValidateHoneytag(presence.Bee, handleParts.Honeytag)
}

// synthesizeHandle creates a handle from BID and optional presence
func (r *Resolver) synthesizeHandle(bid string, presence *dht.PresenceRecord) string {
	if presence != nil && presence.Handle != "" {
		return presence.Handle
	}

	// Synthesize handle from BID
	// For now, use a placeholder nickname. In full implementation, would extract from BID
	return fmt.Sprintf("unknown~%s", r.extractHoneytag(bid))
}

// extractHoneytag extracts honeytag from BID (simplified implementation)
func (r *Resolver) extractHoneytag(bid string) string {
	// This is a placeholder - in full implementation would properly parse BID
	return "placeholder"
}

// extractAddresses extracts addresses from presence record
func (r *Resolver) extractAddresses(presence *dht.PresenceRecord) []string {
	if presence == nil {
		return []string{}
	}
	return presence.Addrs
}

// isValidNickname validates nickname format as specified
func isValidNickname(nickname string) bool {
	if len(nickname) < 3 || len(nickname) > 32 {
		return false
	}

	for _, r := range nickname {
		if !unicode.IsLower(r) && !unicode.IsDigit(r) && r != '-' {
			return false
		}
	}

	return true
}
