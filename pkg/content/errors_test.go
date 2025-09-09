package content

import (
	"errors"
	"testing"
	"time"

	"github.com/WebFirstLanguage/beenet/pkg/wire"
)

func TestContentError(t *testing.T) {
	// Test basic error creation
	err := NewNetworkError("connection failed", "test-provider", errors.New("network down"))

	if err.Code != ErrCodeNetworkFailure {
		t.Errorf("Expected code %s, got %s", ErrCodeNetworkFailure, err.Code)
	}

	if err.Provider != "test-provider" {
		t.Errorf("Expected provider test-provider, got %s", err.Provider)
	}

	if !err.IsRetryable() {
		t.Error("Network error should be retryable")
	}

	// Test error message
	expectedMsg := "content error NETWORK_FAILURE: connection failed"
	if err.Error() != expectedMsg {
		t.Errorf("Expected message %s, got %s", expectedMsg, err.Error())
	}

	// Test unwrapping
	if err.Unwrap().Error() != "network down" {
		t.Error("Error unwrapping failed")
	}
}

func TestTimeoutError(t *testing.T) {
	testCID := NewCID([]byte("test data"))

	err := NewTimeoutError("request timed out", &testCID, "test-provider")

	if err.Code != ErrCodeTimeout {
		t.Errorf("Expected code %s, got %s", ErrCodeTimeout, err.Code)
	}

	if !err.IsRetryable() {
		t.Error("Timeout error should be retryable")
	}

	if err.CID == nil || !err.CID.Equals(testCID) {
		t.Error("CID not properly set in timeout error")
	}
}

func TestIntegrityError(t *testing.T) {
	testCID := NewCID([]byte("test data"))
	cause := errors.New("hash mismatch")

	err := NewIntegrityError("chunk integrity failed", &testCID, cause)

	if err.Code != ErrCodeIntegrityFailure {
		t.Errorf("Expected code %s, got %s", ErrCodeIntegrityFailure, err.Code)
	}

	if err.IsRetryable() {
		t.Error("Integrity error should not be retryable")
	}

	if err.Unwrap() != cause {
		t.Error("Cause not properly wrapped")
	}
}

func TestErrorClassification(t *testing.T) {
	networkErr := NewNetworkError("network failed", "provider", nil)
	timeoutErr := NewTimeoutError("timeout", nil, "provider")
	integrityErr := NewIntegrityError("integrity failed", nil, nil)

	// Test network error classification
	if !IsNetworkError(networkErr) {
		t.Error("Network error not classified correctly")
	}

	if IsTimeoutError(networkErr) {
		t.Error("Network error incorrectly classified as timeout")
	}

	// Test timeout error classification
	if !IsTimeoutError(timeoutErr) {
		t.Error("Timeout error not classified correctly")
	}

	if IsNetworkError(timeoutErr) {
		t.Error("Timeout error incorrectly classified as network")
	}

	// Test integrity error classification
	if !IsIntegrityError(integrityErr) {
		t.Error("Integrity error not classified correctly")
	}

	if IsRetryableError(integrityErr) {
		t.Error("Integrity error should not be retryable")
	}

	// Test retryable classification
	if !IsRetryableError(networkErr) {
		t.Error("Network error should be retryable")
	}

	if !IsRetryableError(timeoutErr) {
		t.Error("Timeout error should be retryable")
	}
}

func TestWrapWireError(t *testing.T) {
	testCID := NewCID([]byte("test data"))

	testCases := []struct {
		name         string
		wireCode     uint16
		expectedCode string
		retryable    bool
	}{
		{
			name:         "not_found",
			wireCode:     404,
			expectedCode: ErrCodeChunkNotFound,
			retryable:    true,
		},
		{
			name:         "rate_limit",
			wireCode:     429,
			expectedCode: ErrCodeRateLimit,
			retryable:    true,
		},
		{
			name:         "server_error",
			wireCode:     500,
			expectedCode: ErrCodeNetworkFailure,
			retryable:    true,
		},
		{
			name:         "bad_request",
			wireCode:     400,
			expectedCode: ErrCodeInvalidRequest,
			retryable:    false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			wireErr := &wire.Error{
				Code:   tc.wireCode,
				Reason: "test error",
			}

			contentErr := WrapWireError(wireErr, &testCID, "test-provider")

			if contentErr.Code != tc.expectedCode {
				t.Errorf("Expected code %s, got %s", tc.expectedCode, contentErr.Code)
			}

			if contentErr.IsRetryable() != tc.retryable {
				t.Errorf("Expected retryable %v, got %v", tc.retryable, contentErr.IsRetryable())
			}

			if contentErr.Provider != "test-provider" {
				t.Errorf("Expected provider test-provider, got %s", contentErr.Provider)
			}

			if contentErr.CID == nil || !contentErr.CID.Equals(testCID) {
				t.Error("CID not properly set")
			}
		})
	}
}

func TestErrorStats(t *testing.T) {
	stats := NewErrorStats()

	// Test initial state
	if stats.GetTotalErrors() != 0 {
		t.Error("Initial error count should be 0")
	}

	// Record some errors
	networkErr := NewNetworkError("network failed", "provider1", nil)
	timeoutErr := NewTimeoutError("timeout", nil, "provider2")
	integrityErr := NewIntegrityError("integrity failed", nil, nil)

	stats.RecordError(networkErr)
	stats.RecordError(timeoutErr)
	stats.RecordError(integrityErr)

	// Check counts
	if stats.NetworkErrors != 1 {
		t.Errorf("Expected 1 network error, got %d", stats.NetworkErrors)
	}

	if stats.TimeoutErrors != 1 {
		t.Errorf("Expected 1 timeout error, got %d", stats.TimeoutErrors)
	}

	if stats.IntegrityErrors != 1 {
		t.Errorf("Expected 1 integrity error, got %d", stats.IntegrityErrors)
	}

	if stats.GetTotalErrors() != 3 {
		t.Errorf("Expected 3 total errors, got %d", stats.GetTotalErrors())
	}

	// Check provider stats
	if stats.ErrorsByProvider["provider1"] != 1 {
		t.Errorf("Expected 1 error for provider1, got %d", stats.ErrorsByProvider["provider1"])
	}

	if stats.ErrorsByProvider["provider2"] != 1 {
		t.Errorf("Expected 1 error for provider2, got %d", stats.ErrorsByProvider["provider2"])
	}

	// Check most problematic provider
	provider, count := stats.GetMostProblematicProvider()
	if count != 1 {
		t.Errorf("Expected max error count 1, got %d", count)
	}

	if provider != "provider1" && provider != "provider2" {
		t.Errorf("Expected provider1 or provider2, got %s", provider)
	}

	// Check last error
	if stats.LastError == nil {
		t.Error("Last error should be set")
	}

	if stats.LastError.Code != ErrCodeIntegrityFailure {
		t.Errorf("Expected last error to be integrity failure, got %s", stats.LastError.Code)
	}

	// Test with multiple errors from same provider
	stats.RecordError(NewNetworkError("another network error", "provider1", nil))

	provider, count = stats.GetMostProblematicProvider()
	if provider != "provider1" || count != 2 {
		t.Errorf("Expected provider1 with 2 errors, got %s with %d errors", provider, count)
	}
}

func TestRateLimitError(t *testing.T) {
	retryAfter := 30 * time.Second
	err := NewRateLimitError("test-provider", retryAfter)

	if err.Code != ErrCodeRateLimit {
		t.Errorf("Expected code %s, got %s", ErrCodeRateLimit, err.Code)
	}

	if !err.IsRetryable() {
		t.Error("Rate limit error should be retryable")
	}

	expectedMsg := "rate limited, retry after 30s"
	if err.Message != expectedMsg {
		t.Errorf("Expected message %s, got %s", expectedMsg, err.Message)
	}
}

func TestProviderNotFoundError(t *testing.T) {
	testCID := NewCID([]byte("test data"))
	err := NewProviderNotFoundError(&testCID)

	if err.Code != ErrCodeProviderNotFound {
		t.Errorf("Expected code %s, got %s", ErrCodeProviderNotFound, err.Code)
	}

	if !err.IsRetryable() {
		t.Error("Provider not found error should be retryable")
	}

	if err.CID == nil || !err.CID.Equals(testCID) {
		t.Error("CID not properly set")
	}
}
