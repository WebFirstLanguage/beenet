// Package cborcanon provides canonical CBOR encoding helpers for Beenet protocol.
// Implements CTAP2-style deterministic encoding as specified in §18.
package cborcanon

import (
	"bytes"
	"fmt"
	"sort"

	"github.com/fxamacker/cbor/v2"
)

// CanonicalMode creates a CBOR encoding mode with canonical settings
// as specified in §18: deterministic key order, no floating types, integer timestamps
var CanonicalMode cbor.EncMode

func init() {
	var err error
	CanonicalMode, err = cbor.CanonicalEncOptions().EncMode()
	if err != nil {
		panic(fmt.Sprintf("failed to create canonical CBOR mode: %v", err))
	}
}

// Marshal encodes v into canonical CBOR format
func Marshal(v interface{}) ([]byte, error) {
	return CanonicalMode.Marshal(v)
}

// Unmarshal decodes canonical CBOR data into v
func Unmarshal(data []byte, v interface{}) error {
	return cbor.Unmarshal(data, v)
}

// MarshalToBytes is a convenience function that returns canonical CBOR bytes
func MarshalToBytes(v interface{}) []byte {
	data, err := Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("canonical CBOR marshal failed: %v", err))
	}
	return data
}

// CanonicalBytes ensures the input bytes represent canonical CBOR
// by unmarshaling and re-marshaling in canonical form
func CanonicalBytes(data []byte) ([]byte, error) {
	var v interface{}
	if err := Unmarshal(data, &v); err != nil {
		return nil, fmt.Errorf("invalid CBOR: %w", err)
	}
	return Marshal(v)
}

// IsCanonical checks if the given CBOR bytes are in canonical form
func IsCanonical(data []byte) bool {
	canonical, err := CanonicalBytes(data)
	if err != nil {
		return false
	}
	return bytes.Equal(data, canonical)
}

// SortedMap represents a map with deterministic key ordering for canonical encoding
type SortedMap struct {
	Keys   []string
	Values map[string]interface{}
}

// NewSortedMap creates a new SortedMap from a regular map
func NewSortedMap(m map[string]interface{}) *SortedMap {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	
	return &SortedMap{
		Keys:   keys,
		Values: m,
	}
}

// MarshalCBOR implements custom CBOR marshaling for deterministic key order
func (sm *SortedMap) MarshalCBOR() ([]byte, error) {
	orderedMap := make(map[string]interface{})
	for _, key := range sm.Keys {
		orderedMap[key] = sm.Values[key]
	}
	return CanonicalMode.Marshal(orderedMap)
}

// UnmarshalCBOR implements custom CBOR unmarshaling
func (sm *SortedMap) UnmarshalCBOR(data []byte) error {
	var m map[string]interface{}
	if err := cbor.Unmarshal(data, &m); err != nil {
		return err
	}
	
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	
	sm.Keys = keys
	sm.Values = m
	return nil
}

// Deterministic encoding helpers for common Beenet types

// EncodeForSigning encodes a structure for signing, excluding the signature field
func EncodeForSigning(v interface{}, excludeFields ...string) ([]byte, error) {
	// Convert to map for field exclusion
	data, err := Marshal(v)
	if err != nil {
		return nil, err
	}
	
	var m map[string]interface{}
	if err := Unmarshal(data, &m); err != nil {
		return nil, err
	}
	
	// Remove excluded fields (typically "sig")
	for _, field := range excludeFields {
		delete(m, field)
	}
	
	// Re-encode canonically
	return Marshal(NewSortedMap(m))
}

// ValidateCanonical validates that the given data is canonical CBOR
func ValidateCanonical(data []byte) error {
	if !IsCanonical(data) {
		return fmt.Errorf("data is not in canonical CBOR form")
	}
	return nil
}
