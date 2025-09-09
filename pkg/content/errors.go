package content

import (
	"errors"
	"fmt"
	"time"

	"github.com/WebFirstLanguage/beenet/pkg/wire"
)

// ContentError represents content-specific errors
type ContentError struct {
	Code      string    `json:"code"`
	Message   string    `json:"message"`
	CID       *CID      `json:"cid,omitempty"`
	Provider  string    `json:"provider,omitempty"`
	Timestamp time.Time `json:"timestamp"`
	Retryable bool      `json:"retryable"`
	Cause     error     `json:"-"` // Original error, not serialized
}

// Error implements the error interface
func (e *ContentError) Error() string {
	if e.CID != nil {
		return fmt.Sprintf("content error %s: %s (CID: %s)", e.Code, e.Message, e.CID.String)
	}
	return fmt.Sprintf("content error %s: %s", e.Code, e.Message)
}

// Unwrap returns the underlying error
func (e *ContentError) Unwrap() error {
	return e.Cause
}

// IsRetryable returns whether this error suggests retrying
func (e *ContentError) IsRetryable() bool {
	return e.Retryable
}

// Error codes for content operations
const (
	ErrCodeNetworkFailure    = "NETWORK_FAILURE"
	ErrCodeTimeout           = "TIMEOUT"
	ErrCodeIntegrityFailure  = "INTEGRITY_FAILURE"
	ErrCodeProviderNotFound  = "PROVIDER_NOT_FOUND"
	ErrCodeChunkNotFound     = "CHUNK_NOT_FOUND"
	ErrCodeManifestInvalid   = "MANIFEST_INVALID"
	ErrCodeCIDInvalid        = "CID_INVALID"
	ErrCodeCorruptedData     = "CORRUPTED_DATA"
	ErrCodeInsufficientSpace = "INSUFFICIENT_SPACE"
	ErrCodePermissionDenied  = "PERMISSION_DENIED"
	ErrCodeRateLimit         = "RATE_LIMIT"
	ErrCodeVersionMismatch   = "VERSION_MISMATCH"
	ErrCodeConcurrencyLimit  = "CONCURRENCY_LIMIT"
	ErrCodeInvalidRequest    = "INVALID_REQUEST"
)

// Error constructors

// NewNetworkError creates a network-related error
func NewNetworkError(message string, provider string, cause error) *ContentError {
	return &ContentError{
		Code:      ErrCodeNetworkFailure,
		Message:   message,
		Provider:  provider,
		Timestamp: time.Now(),
		Retryable: true,
		Cause:     cause,
	}
}

// NewTimeoutError creates a timeout error
func NewTimeoutError(message string, cid *CID, provider string) *ContentError {
	return &ContentError{
		Code:      ErrCodeTimeout,
		Message:   message,
		CID:       cid,
		Provider:  provider,
		Timestamp: time.Now(),
		Retryable: true,
	}
}

// NewIntegrityError creates an integrity verification error
func NewIntegrityError(message string, cid *CID, cause error) *ContentError {
	return &ContentError{
		Code:      ErrCodeIntegrityFailure,
		Message:   message,
		CID:       cid,
		Timestamp: time.Now(),
		Retryable: false,
		Cause:     cause,
	}
}

// NewProviderNotFoundError creates a provider not found error
func NewProviderNotFoundError(cid *CID) *ContentError {
	return &ContentError{
		Code:      ErrCodeProviderNotFound,
		Message:   "no providers found for content",
		CID:       cid,
		Timestamp: time.Now(),
		Retryable: true,
	}
}

// NewChunkNotFoundError creates a chunk not found error
func NewChunkNotFoundError(cid *CID, provider string) *ContentError {
	return &ContentError{
		Code:      ErrCodeChunkNotFound,
		Message:   "chunk not found",
		CID:       cid,
		Provider:  provider,
		Timestamp: time.Now(),
		Retryable: true,
	}
}

// NewManifestInvalidError creates a manifest validation error
func NewManifestInvalidError(message string, cause error) *ContentError {
	return &ContentError{
		Code:      ErrCodeManifestInvalid,
		Message:   message,
		Timestamp: time.Now(),
		Retryable: false,
		Cause:     cause,
	}
}

// NewCIDInvalidError creates a CID validation error
func NewCIDInvalidError(message string, cause error) *ContentError {
	return &ContentError{
		Code:      ErrCodeCIDInvalid,
		Message:   message,
		Timestamp: time.Now(),
		Retryable: false,
		Cause:     cause,
	}
}

// NewCorruptedDataError creates a data corruption error
func NewCorruptedDataError(message string, cid *CID, cause error) *ContentError {
	return &ContentError{
		Code:      ErrCodeCorruptedData,
		Message:   message,
		CID:       cid,
		Timestamp: time.Now(),
		Retryable: false,
		Cause:     cause,
	}
}

// NewRateLimitError creates a rate limit error
func NewRateLimitError(provider string, retryAfter time.Duration) *ContentError {
	message := fmt.Sprintf("rate limited, retry after %v", retryAfter)
	return &ContentError{
		Code:      ErrCodeRateLimit,
		Message:   message,
		Provider:  provider,
		Timestamp: time.Now(),
		Retryable: true,
	}
}

// NewInvalidRequestError creates an invalid request error
func NewInvalidRequestError(message string, cause error) *ContentError {
	return &ContentError{
		Code:      ErrCodeInvalidRequest,
		Message:   message,
		Timestamp: time.Now(),
		Retryable: false,
		Cause:     cause,
	}
}

// Error classification functions

// IsNetworkError checks if an error is network-related
func IsNetworkError(err error) bool {
	var contentErr *ContentError
	if errors.As(err, &contentErr) {
		return contentErr.Code == ErrCodeNetworkFailure
	}
	return false
}

// IsTimeoutError checks if an error is timeout-related
func IsTimeoutError(err error) bool {
	var contentErr *ContentError
	if errors.As(err, &contentErr) {
		return contentErr.Code == ErrCodeTimeout
	}
	return false
}

// IsIntegrityError checks if an error is integrity-related
func IsIntegrityError(err error) bool {
	var contentErr *ContentError
	if errors.As(err, &contentErr) {
		return contentErr.Code == ErrCodeIntegrityFailure
	}
	return false
}

// IsRetryableError checks if an error suggests retrying
func IsRetryableError(err error) bool {
	var contentErr *ContentError
	if errors.As(err, &contentErr) {
		return contentErr.Retryable
	}
	return false
}

// WrapWireError converts a wire protocol error to a content error
func WrapWireError(wireErr *wire.Error, cid *CID, provider string) *ContentError {
	var code string
	var retryable bool

	switch wireErr.Code {
	case 404: // Not found
		code = ErrCodeChunkNotFound
		retryable = true
	case 429: // Rate limit
		code = ErrCodeRateLimit
		retryable = true
	case 500: // Internal server error
		code = ErrCodeNetworkFailure
		retryable = true
	case 400: // Bad request
		code = ErrCodeInvalidRequest
		retryable = false
	default:
		code = ErrCodeNetworkFailure
		retryable = wireErr.IsRetryable()
	}

	return &ContentError{
		Code:      code,
		Message:   wireErr.Reason,
		CID:       cid,
		Provider:  provider,
		Timestamp: time.Now(),
		Retryable: retryable,
		Cause:     wireErr,
	}
}

// ErrorStats tracks error statistics
type ErrorStats struct {
	NetworkErrors    uint64            `json:"network_errors"`
	TimeoutErrors    uint64            `json:"timeout_errors"`
	IntegrityErrors  uint64            `json:"integrity_errors"`
	ProviderErrors   uint64            `json:"provider_errors"`
	CorruptionErrors uint64            `json:"corruption_errors"`
	RateLimitErrors  uint64            `json:"rate_limit_errors"`
	ErrorsByProvider map[string]uint64 `json:"errors_by_provider"`
	LastError        *ContentError     `json:"last_error,omitempty"`
	LastErrorTime    time.Time         `json:"last_error_time"`
}

// NewErrorStats creates a new error statistics tracker
func NewErrorStats() *ErrorStats {
	return &ErrorStats{
		ErrorsByProvider: make(map[string]uint64),
	}
}

// RecordError records an error in the statistics
func (es *ErrorStats) RecordError(err *ContentError) {
	es.LastError = err
	es.LastErrorTime = time.Now()

	switch err.Code {
	case ErrCodeNetworkFailure:
		es.NetworkErrors++
	case ErrCodeTimeout:
		es.TimeoutErrors++
	case ErrCodeIntegrityFailure:
		es.IntegrityErrors++
	case ErrCodeProviderNotFound, ErrCodeChunkNotFound:
		es.ProviderErrors++
	case ErrCodeCorruptedData:
		es.CorruptionErrors++
	case ErrCodeRateLimit:
		es.RateLimitErrors++
	}

	if err.Provider != "" {
		es.ErrorsByProvider[err.Provider]++
	}
}

// GetTotalErrors returns the total number of errors recorded
func (es *ErrorStats) GetTotalErrors() uint64 {
	return es.NetworkErrors + es.TimeoutErrors + es.IntegrityErrors +
		es.ProviderErrors + es.CorruptionErrors + es.RateLimitErrors
}

// GetMostProblematicProvider returns the provider with the most errors
func (es *ErrorStats) GetMostProblematicProvider() (string, uint64) {
	var maxProvider string
	var maxErrors uint64

	for provider, count := range es.ErrorsByProvider {
		if count > maxErrors {
			maxErrors = count
			maxProvider = provider
		}
	}

	return maxProvider, maxErrors
}
