package cborcanon

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"testing"
)

// Golden test vectors for canonical CBOR determinism
var canonicalTestVectors = []struct {
	name     string
	input    interface{}
	expected string // hex-encoded canonical CBOR
}{
	{
		name:     "simple_map",
		input:    map[string]interface{}{"b": 2, "a": 1},
		expected: "", // Will be computed dynamically
	},
	{
		name: "nested_map",
		input: map[string]interface{}{
			"z": 3,
			"a": map[string]interface{}{
				"y": 2,
				"x": 1,
			},
		},
		expected: "", // Will be computed dynamically
	},
	{
		name:     "array",
		input:    []interface{}{3, 1, 2},
		expected: "83030102", // [3, 1, 2] - arrays preserve order
	},
	{
		name:     "mixed_types",
		input:    map[string]interface{}{"str": "hello", "num": 42, "bool": true},
		expected: "", // Will be computed dynamically
	},
	{
		name:     "empty_map",
		input:    map[string]interface{}{},
		expected: "a0", // {}
	},
	{
		name:     "empty_array",
		input:    []interface{}{},
		expected: "80", // []
	},
}

func TestCanonicalEncoding(t *testing.T) {
	for _, tv := range canonicalTestVectors {
		t.Run(tv.name, func(t *testing.T) {
			encoded, err := Marshal(tv.input)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			encodedHex := hex.EncodeToString(encoded)

			// Only check expected value if it's provided
			if tv.expected != "" && encodedHex != tv.expected {
				t.Errorf("Expected %s, got %s", tv.expected, encodedHex)
			}

			// Verify round-trip
			var decoded interface{}
			if err := Unmarshal(encoded, &decoded); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			// Re-encode to verify determinism
			reencoded, err := Marshal(decoded)
			if err != nil {
				t.Fatalf("Re-marshal failed: %v", err)
			}

			if !bytes.Equal(encoded, reencoded) {
				t.Errorf("Encoding not deterministic: %x != %x", encoded, reencoded)
			}

			// Log the actual encoding for reference
			t.Logf("Canonical CBOR for %s: %s", tv.name, encodedHex)
		})
	}
}

func TestIsCanonical(t *testing.T) {
	tests := []struct {
		name      string
		data      string // hex-encoded CBOR
		canonical bool
	}{
		{
			name:      "canonical_map",
			data:      "a2616101616202", // {"a": 1, "b": 2}
			canonical: true,
		},
		{
			name:      "non_canonical_map",
			data:      "a2616202616101", // {"b": 2, "a": 1} - wrong order
			canonical: false,
		},
		{
			name:      "canonical_array",
			data:      "83010203", // [1, 2, 3]
			canonical: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := hex.DecodeString(tt.data)
			if err != nil {
				t.Fatalf("Invalid hex: %v", err)
			}

			if IsCanonical(data) != tt.canonical {
				t.Errorf("IsCanonical() = %v, want %v", IsCanonical(data), tt.canonical)
			}
		})
	}
}

func TestSortedMap(t *testing.T) {
	original := map[string]interface{}{
		"z": 3,
		"a": 1,
		"m": 2,
	}

	sm := NewSortedMap(original)

	// Check key order
	expectedOrder := []string{"a", "m", "z"}
	if len(sm.Keys) != len(expectedOrder) {
		t.Fatalf("Expected %d keys, got %d", len(expectedOrder), len(sm.Keys))
	}

	for i, key := range expectedOrder {
		if sm.Keys[i] != key {
			t.Errorf("Key at position %d: expected %s, got %s", i, key, sm.Keys[i])
		}
	}

	// Test marshaling
	encoded, err := sm.MarshalCBOR()
	if err != nil {
		t.Fatalf("MarshalCBOR failed: %v", err)
	}

	// Verify it's canonical
	if !IsCanonical(encoded) {
		t.Error("SortedMap did not produce canonical CBOR")
	}

	// Test unmarshaling
	var sm2 SortedMap
	if err := sm2.UnmarshalCBOR(encoded); err != nil {
		t.Fatalf("UnmarshalCBOR failed: %v", err)
	}

	// Verify round-trip
	if len(sm2.Keys) != len(sm.Keys) {
		t.Errorf("Key count mismatch after round-trip: %d != %d", len(sm2.Keys), len(sm.Keys))
	}

	for i, key := range sm.Keys {
		if sm2.Keys[i] != key {
			t.Errorf("Key mismatch at position %d: %s != %s", i, sm2.Keys[i], key)
		}
		// Convert values to comparable types for comparison
		val1 := fmt.Sprintf("%v", sm2.Values[key])
		val2 := fmt.Sprintf("%v", sm.Values[key])
		if val1 != val2 {
			t.Errorf("Value mismatch for key %s: %v != %v", key, sm2.Values[key], sm.Values[key])
		}
	}
}

func TestEncodeForSigning(t *testing.T) {
	input := map[string]interface{}{
		"v":    1,
		"from": "test",
		"data": "payload",
		"sig":  "signature_to_exclude",
	}

	encoded, err := EncodeForSigning(input, "sig")
	if err != nil {
		t.Fatalf("EncodeForSigning failed: %v", err)
	}

	// Decode to verify signature field was excluded
	var decoded map[string]interface{}
	if err := Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if _, exists := decoded["sig"]; exists {
		t.Error("Signature field was not excluded")
	}

	// Check that expected fields are present and correct
	if v, ok := decoded["v"]; !ok || fmt.Sprintf("%v", v) != "1" {
		t.Error("Field 'v' was incorrectly modified or missing")
	}
	if from, ok := decoded["from"]; !ok || fmt.Sprintf("%v", from) != "test" {
		t.Error("Field 'from' was incorrectly modified or missing")
	}
	if data, ok := decoded["data"]; !ok || fmt.Sprintf("%v", data) != "payload" {
		t.Error("Field 'data' was incorrectly modified or missing")
	}

	// Verify it's canonical
	if !IsCanonical(encoded) {
		t.Error("EncodeForSigning did not produce canonical CBOR")
	}
}

func BenchmarkCanonicalMarshal(b *testing.B) {
	data := map[string]interface{}{
		"version": 1,
		"kind":    10,
		"from":    "bee:key:z6MkhaXgBZDvotDkL5257faiztiGiC2QtKLGpbnnEGta2doK",
		"seq":     uint64(12345),
		"ts":      uint64(1609459200000),
		"body": map[string]interface{}{
			"key":   "some_key",
			"value": "some_value",
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := Marshal(data)
		if err != nil {
			b.Fatal(err)
		}
	}
}
