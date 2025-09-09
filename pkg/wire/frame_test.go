package wire

import (
	"crypto/ed25519"
	"crypto/rand"
	"testing"
	"time"

	"github.com/WebFirstLanguage/beenet/pkg/constants"
)

func TestBaseFrame_SignAndVerify(t *testing.T) {
	// Generate test key pair
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	// Create test frame
	frame := NewBaseFrame(constants.KindPing, "test-bid", 1, &PingBody{
		Token: []byte("testtoken"),
	})

	// Sign the frame
	if err := frame.Sign(privateKey); err != nil {
		t.Fatalf("Failed to sign frame: %v", err)
	}

	// Verify signature should succeed
	if err := frame.Verify(publicKey); err != nil {
		t.Errorf("Signature verification failed: %v", err)
	}

	// Modify frame and verify should fail
	originalSeq := frame.Seq
	frame.Seq = 999
	if err := frame.Verify(publicKey); err == nil {
		t.Error("Expected signature verification to fail after modification")
	}

	// Restore and verify should succeed again
	frame.Seq = originalSeq
	if err := frame.Verify(publicKey); err != nil {
		t.Errorf("Signature verification failed after restoration: %v", err)
	}
}

func TestBaseFrame_MarshalUnmarshal(t *testing.T) {
	// Generate test key pair
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	// Create and sign test frame
	original := NewBaseFrame(constants.KindDHTGet, "test-bid", 42, &DHTGetBody{
		Key: []byte("test-key-32-bytes-long-exactly!!"),
	})

	if err := original.Sign(privateKey); err != nil {
		t.Fatalf("Failed to sign frame: %v", err)
	}

	// Marshal
	data, err := original.Marshal()
	if err != nil {
		t.Fatalf("Failed to marshal frame: %v", err)
	}

	// Unmarshal
	var decoded BaseFrame
	if err := decoded.Unmarshal(data); err != nil {
		t.Fatalf("Failed to unmarshal frame: %v", err)
	}

	// Compare fields
	if decoded.V != original.V {
		t.Errorf("Version mismatch: %d != %d", decoded.V, original.V)
	}
	if decoded.Kind != original.Kind {
		t.Errorf("Kind mismatch: %d != %d", decoded.Kind, original.Kind)
	}
	if decoded.From != original.From {
		t.Errorf("From mismatch: %s != %s", decoded.From, original.From)
	}
	if decoded.Seq != original.Seq {
		t.Errorf("Seq mismatch: %d != %d", decoded.Seq, original.Seq)
	}
	if decoded.TS != original.TS {
		t.Errorf("TS mismatch: %d != %d", decoded.TS, original.TS)
	}

	// Compare signatures
	if len(decoded.Sig) != len(original.Sig) {
		t.Errorf("Signature length mismatch: %d != %d", len(decoded.Sig), len(original.Sig))
	}
	for i, b := range original.Sig {
		if decoded.Sig[i] != b {
			t.Errorf("Signature byte %d mismatch: %02x != %02x", i, decoded.Sig[i], b)
		}
	}
}

func TestBaseFrame_Validate(t *testing.T) {
	tests := []struct {
		name      string
		frame     *BaseFrame
		wantError bool
		errorCode uint16
	}{
		{
			name: "valid_frame",
			frame: &BaseFrame{
				V:    constants.ProtocolVersion,
				Kind: constants.KindPing,
				From: "test-bid",
				Seq:  1,
				TS:   uint64(time.Now().UnixMilli()),
				Body: &PingBody{Token: []byte("test")},
				Sig:  []byte("fake-signature"),
			},
			wantError: false,
		},
		{
			name: "wrong_version",
			frame: &BaseFrame{
				V:    99,
				Kind: constants.KindPing,
				From: "test-bid",
				Seq:  1,
				TS:   uint64(time.Now().UnixMilli()),
				Body: &PingBody{Token: []byte("test")},
				Sig:  []byte("fake-signature"),
			},
			wantError: true,
			errorCode: constants.ErrorVersionMismatch,
		},
		{
			name: "missing_from",
			frame: &BaseFrame{
				V:    constants.ProtocolVersion,
				Kind: constants.KindPing,
				From: "",
				Seq:  1,
				TS:   uint64(time.Now().UnixMilli()),
				Body: &PingBody{Token: []byte("test")},
				Sig:  []byte("fake-signature"),
			},
			wantError: true,
			errorCode: constants.ErrorInvalidSig,
		},
		{
			name: "missing_signature",
			frame: &BaseFrame{
				V:    constants.ProtocolVersion,
				Kind: constants.KindPing,
				From: "test-bid",
				Seq:  1,
				TS:   uint64(time.Now().UnixMilli()),
				Body: &PingBody{Token: []byte("test")},
				Sig:  nil,
			},
			wantError: true,
			errorCode: constants.ErrorInvalidSig,
		},
		{
			name: "timestamp_too_far_future",
			frame: &BaseFrame{
				V:    constants.ProtocolVersion,
				Kind: constants.KindPing,
				From: "test-bid",
				Seq:  1,
				TS:   uint64(time.Now().Add(10 * time.Minute).UnixMilli()),
				Body: &PingBody{Token: []byte("test")},
				Sig:  []byte("fake-signature"),
			},
			wantError: true,
			errorCode: constants.ErrorVersionMismatch,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.frame.Validate()
			if tt.wantError {
				if err == nil {
					t.Error("Expected validation error, got nil")
					return
				}
				if wireErr, ok := err.(*Error); ok {
					if wireErr.Code != tt.errorCode {
						t.Errorf("Expected error code %d, got %d", tt.errorCode, wireErr.Code)
					}
				} else {
					t.Errorf("Expected wire.Error, got %T", err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no validation error, got: %v", err)
				}
			}
		})
	}
}

func TestFrameHelpers(t *testing.T) {
	// Test PING frame
	pingFrame := NewPingFrame("test-bid", 1, []byte("testtoken"))
	if pingFrame.Kind != constants.KindPing {
		t.Errorf("Expected PING kind %d, got %d", constants.KindPing, pingFrame.Kind)
	}
	if !pingFrame.IsKind(constants.KindPing) {
		t.Error("IsKind should return true for PING frame")
	}

	// Test PONG frame
	pongFrame := NewPongFrame("test-bid", 2, []byte("testtoken"))
	if pongFrame.Kind != constants.KindPong {
		t.Errorf("Expected PONG kind %d, got %d", constants.KindPong, pongFrame.Kind)
	}

	// Test DHT GET frame
	dhtGetFrame := NewDHTGetFrame("test-bid", 3, []byte("test-key-32-bytes-long-exactly!!"))
	if dhtGetFrame.Kind != constants.KindDHTGet {
		t.Errorf("Expected DHT_GET kind %d, got %d", constants.KindDHTGet, dhtGetFrame.Kind)
	}

	// Test timestamp conversion
	now := time.Now()
	frame := NewBaseFrame(constants.KindPing, "test", 1, nil)
	frameTime := frame.GetTimestamp()
	
	// Should be within 1 second of now
	if frameTime.Sub(now).Abs() > time.Second {
		t.Errorf("Frame timestamp %v too far from now %v", frameTime, now)
	}
}

func BenchmarkBaseFrame_Sign(b *testing.B) {
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		b.Fatalf("Failed to generate key pair: %v", err)
	}

	frame := NewBaseFrame(constants.KindPing, "test-bid", 1, &PingBody{
		Token: []byte("testtoken"),
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := frame.Sign(privateKey); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkBaseFrame_Verify(b *testing.B) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		b.Fatalf("Failed to generate key pair: %v", err)
	}

	frame := NewBaseFrame(constants.KindPing, "test-bid", 1, &PingBody{
		Token: []byte("testtoken"),
	})

	if err := frame.Sign(privateKey); err != nil {
		b.Fatalf("Failed to sign frame: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := frame.Verify(publicKey); err != nil {
			b.Fatal(err)
		}
	}
}
