package wire

import (
	"testing"
	"time"

	"github.com/WebFirstLanguage/beenet/pkg/constants"
)

func TestSWIMMessageBodies(t *testing.T) {
	tests := []struct {
		name     string
		kind     uint16
		body     interface{}
		wantType string
	}{
		{
			name: "swim_ping",
			kind: constants.KindSWIMPing,
			body: &SWIMPingBody{
				Target: "test-target-bid",
				SeqNo:  12345,
			},
			wantType: "*wire.SWIMPingBody",
		},
		{
			name: "swim_ping_req",
			kind: constants.KindSWIMPingReq,
			body: &SWIMPingReqBody{
				Target:    "test-target-bid",
				SeqNo:     12345,
				Requestor: "test-requestor-bid",
				Timeout:   5000, // 5 seconds in milliseconds
			},
			wantType: "*wire.SWIMPingReqBody",
		},
		{
			name: "swim_ack",
			kind: constants.KindSWIMAck,
			body: &SWIMAckBody{
				SeqNo: 12345,
			},
			wantType: "*wire.SWIMAckBody",
		},
		{
			name: "swim_suspect",
			kind: constants.KindSWIMSuspect,
			body: &SWIMSuspectBody{
				Target:      "test-target-bid",
				Incarnation: 1,
			},
			wantType: "*wire.SWIMSuspectBody",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			frame := NewBaseFrame(tt.kind, "test-sender", 1, tt.body)

			if frame.Kind != tt.kind {
				t.Errorf("Expected kind %d, got %d", tt.kind, frame.Kind)
			}

			if frame.Body == nil {
				t.Error("Expected body to be set")
			}

			// Check that the body type matches expected
			bodyType := getTypeName(frame.Body)
			if bodyType != tt.wantType {
				t.Errorf("Expected body type %s, got %s", tt.wantType, bodyType)
			}
		})
	}
}

func TestGossipMessageBodies(t *testing.T) {
	tests := []struct {
		name     string
		kind     uint16
		body     interface{}
		wantType string
	}{
		{
			name: "gossip_ihave",
			kind: constants.KindGossipIHave,
			body: &GossipIHaveBody{
				Topic:      "test-topic-id",
				MessageIDs: []string{"msg1", "msg2", "msg3"},
			},
			wantType: "*wire.GossipIHaveBody",
		},
		{
			name: "gossip_iwant",
			kind: constants.KindGossipIWant,
			body: &GossipIWantBody{
				MessageIDs: []string{"msg1", "msg2"},
			},
			wantType: "*wire.GossipIWantBody",
		},
		{
			name: "gossip_graft",
			kind: constants.KindGossipGraft,
			body: &GossipGraftBody{
				Topic: "test-topic-id",
			},
			wantType: "*wire.GossipGraftBody",
		},
		{
			name: "gossip_prune",
			kind: constants.KindGossipPrune,
			body: &GossipPruneBody{
				Topic: "test-topic-id",
				Peers: []string{"peer1", "peer2"},
			},
			wantType: "*wire.GossipPruneBody",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			frame := NewBaseFrame(tt.kind, "test-sender", 1, tt.body)

			if frame.Kind != tt.kind {
				t.Errorf("Expected kind %d, got %d", tt.kind, frame.Kind)
			}

			if frame.Body == nil {
				t.Error("Expected body to be set")
			}

			// Check that the body type matches expected
			bodyType := getTypeName(frame.Body)
			if bodyType != tt.wantType {
				t.Errorf("Expected body type %s, got %s", tt.wantType, bodyType)
			}
		})
	}
}

func TestPubSubMessageEnvelope(t *testing.T) {
	envelope := &PubSubMessageEnvelope{
		MID:     "test-message-id",
		From:    "test-sender-bid",
		Seq:     12345,
		TS:      uint64(time.Now().UnixMilli()),
		Topic:   "test-topic-id",
		Payload: []byte("test message payload"),
		Sig:     []byte("fake-signature"),
	}

	frame := NewBaseFrame(constants.KindPubSubMsg, "test-sender", 1, envelope)

	if frame.Kind != constants.KindPubSubMsg {
		t.Errorf("Expected kind %d, got %d", constants.KindPubSubMsg, frame.Kind)
	}

	if frame.Body == nil {
		t.Error("Expected body to be set")
	}

	bodyType := getTypeName(frame.Body)
	if bodyType != "*wire.PubSubMessageEnvelope" {
		t.Errorf("Expected body type *wire.PubSubMessageEnvelope, got %s", bodyType)
	}
}

func TestSWIMHelperFunctions(t *testing.T) {
	tests := []struct {
		name      string
		frameFunc func() *BaseFrame
		wantKind  uint16
	}{
		{
			name: "swim_ping_frame",
			frameFunc: func() *BaseFrame {
				return NewSWIMPingFrame("test-sender", 1, "test-target", 12345)
			},
			wantKind: constants.KindSWIMPing,
		},
		{
			name: "swim_ping_req_frame",
			frameFunc: func() *BaseFrame {
				return NewSWIMPingReqFrame("test-sender", 1, "test-target", "test-requestor", 12345, 5000)
			},
			wantKind: constants.KindSWIMPingReq,
		},
		{
			name: "swim_ack_frame",
			frameFunc: func() *BaseFrame {
				return NewSWIMAckFrame("test-sender", 1, 12345)
			},
			wantKind: constants.KindSWIMAck,
		},
		{
			name: "swim_suspect_frame",
			frameFunc: func() *BaseFrame {
				return NewSWIMSuspectFrame("test-sender", 1, "test-target", 1)
			},
			wantKind: constants.KindSWIMSuspect,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			frame := tt.frameFunc()

			if frame.Kind != tt.wantKind {
				t.Errorf("Expected kind %d, got %d", tt.wantKind, frame.Kind)
			}

			if frame.From != "test-sender" {
				t.Errorf("Expected from 'test-sender', got '%s'", frame.From)
			}

			if frame.Seq != 1 {
				t.Errorf("Expected seq 1, got %d", frame.Seq)
			}

			if frame.Body == nil {
				t.Error("Expected body to be set")
			}
		})
	}
}

func TestGossipHelperFunctions(t *testing.T) {
	tests := []struct {
		name      string
		frameFunc func() *BaseFrame
		wantKind  uint16
	}{
		{
			name: "gossip_ihave_frame",
			frameFunc: func() *BaseFrame {
				return NewGossipIHaveFrame("test-sender", 1, "test-topic", []string{"msg1", "msg2"})
			},
			wantKind: constants.KindGossipIHave,
		},
		{
			name: "gossip_iwant_frame",
			frameFunc: func() *BaseFrame {
				return NewGossipIWantFrame("test-sender", 1, []string{"msg1", "msg2"})
			},
			wantKind: constants.KindGossipIWant,
		},
		{
			name: "gossip_graft_frame",
			frameFunc: func() *BaseFrame {
				return NewGossipGraftFrame("test-sender", 1, "test-topic")
			},
			wantKind: constants.KindGossipGraft,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			frame := tt.frameFunc()

			if frame.Kind != tt.wantKind {
				t.Errorf("Expected kind %d, got %d", tt.wantKind, frame.Kind)
			}

			if frame.From != "test-sender" {
				t.Errorf("Expected from 'test-sender', got '%s'", frame.From)
			}

			if frame.Body == nil {
				t.Error("Expected body to be set")
			}
		})
	}
}

// Helper function to get type name for testing
func getTypeName(v interface{}) string {
	switch v.(type) {
	case *SWIMPingBody:
		return "*wire.SWIMPingBody"
	case *SWIMPingReqBody:
		return "*wire.SWIMPingReqBody"
	case *SWIMAckBody:
		return "*wire.SWIMAckBody"
	case *SWIMSuspectBody:
		return "*wire.SWIMSuspectBody"
	case *GossipIHaveBody:
		return "*wire.GossipIHaveBody"
	case *GossipIWantBody:
		return "*wire.GossipIWantBody"
	case *GossipGraftBody:
		return "*wire.GossipGraftBody"
	case *GossipPruneBody:
		return "*wire.GossipPruneBody"
	case *PubSubMessageEnvelope:
		return "*wire.PubSubMessageEnvelope"
	default:
		return "unknown"
	}
}
