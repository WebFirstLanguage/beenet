package wire

import (
	"fmt"

	"github.com/WebFirstLanguage/beenet/pkg/constants"
)

// Error represents a Beenet protocol error as specified in §17
type Error struct {
	Code       uint16  `cbor:"code"`                 // Error code
	Reason     string  `cbor:"reason"`               // Human-readable error message
	RetryAfter *uint32 `cbor:"retry_after,omitempty"` // Optional retry delay in seconds
}

// NewError creates a new protocol error
func NewError(code uint16, reason string) *Error {
	return &Error{
		Code:   code,
		Reason: reason,
	}
}

// NewErrorWithRetry creates a new protocol error with retry-after
func NewErrorWithRetry(code uint16, reason string, retryAfter uint32) *Error {
	return &Error{
		Code:       code,
		Reason:     reason,
		RetryAfter: &retryAfter,
	}
}

// Error implements the error interface
func (e *Error) Error() string {
	if e.RetryAfter != nil {
		return fmt.Sprintf("beenet error %d: %s (retry after %ds)", e.Code, e.Reason, *e.RetryAfter)
	}
	return fmt.Sprintf("beenet error %d: %s", e.Code, e.Reason)
}

// IsRetryable returns true if the error suggests retrying
func (e *Error) IsRetryable() bool {
	return e.RetryAfter != nil || e.Code == constants.ErrorRateLimit
}

// ErrorCodeName returns the human-readable name for an error code
func ErrorCodeName(code uint16) string {
	switch code {
	case constants.ErrorInvalidSig:
		return "INVALID_SIG"
	case constants.ErrorNotInSwarm:
		return "NOT_IN_SWARM"
	case constants.ErrorNoProvider:
		return "NO_PROVIDER"
	case constants.ErrorRateLimit:
		return "RATE_LIMIT"
	case constants.ErrorVersionMismatch:
		return "VERSION_MISMATCH"
	case constants.ErrorNameNotFound:
		return "NAME_NOT_FOUND"
	case constants.ErrorNameLeaseExpired:
		return "NAME_LEASE_EXPIRED"
	case constants.ErrorHandleMismatch:
		return "HANDLE_MISMATCH"
	case constants.ErrorNotOwner:
		return "NOT_OWNER"
	case constants.ErrorDelegationMissing:
		return "DELEGATION_MISSING"
	default:
		return fmt.Sprintf("UNKNOWN_%d", code)
	}
}

// Common error constructors

// ErrInvalidSignature creates an invalid signature error
func ErrInvalidSignature(reason string) *Error {
	return NewError(constants.ErrorInvalidSig, reason)
}

// ErrNotInSwarm creates a not-in-swarm error
func ErrNotInSwarm(swarmID string) *Error {
	return NewError(constants.ErrorNotInSwarm, fmt.Sprintf("not a member of swarm %s", swarmID))
}

// ErrNoProvider creates a no-provider error
func ErrNoProvider(key string) *Error {
	return NewError(constants.ErrorNoProvider, fmt.Sprintf("no provider found for %s", key))
}

// ErrRateLimit creates a rate limit error with retry-after
func ErrRateLimit(retryAfter uint32) *Error {
	return NewErrorWithRetry(constants.ErrorRateLimit, "rate limit exceeded", retryAfter)
}

// ErrVersionMismatch creates a version mismatch error
func ErrVersionMismatch(expected, actual uint16) *Error {
	return NewError(constants.ErrorVersionMismatch, 
		fmt.Sprintf("version mismatch: expected %d, got %d", expected, actual))
}

// Honeytag-specific errors

// ErrNameNotFound creates a name-not-found error
func ErrNameNotFound(name string) *Error {
	return NewError(constants.ErrorNameNotFound, fmt.Sprintf("name not found: %s", name))
}

// ErrNameLeaseExpired creates a name-lease-expired error
func ErrNameLeaseExpired(name string) *Error {
	return NewError(constants.ErrorNameLeaseExpired, fmt.Sprintf("name lease expired: %s", name))
}

// ErrHandleMismatch creates a handle-mismatch error
func ErrHandleMismatch(handle, expected string) *Error {
	return NewError(constants.ErrorHandleMismatch, 
		fmt.Sprintf("handle mismatch: %s != %s", handle, expected))
}

// ErrNotOwner creates a not-owner error
func ErrNotOwner(name, owner, requester string) *Error {
	return NewError(constants.ErrorNotOwner, 
		fmt.Sprintf("not owner of %s: owner=%s, requester=%s", name, owner, requester))
}

// ErrDelegationMissing creates a delegation-missing error
func ErrDelegationMissing(owner, device string) *Error {
	return NewError(constants.ErrorDelegationMissing, 
		fmt.Sprintf("no delegation from %s to %s", owner, device))
}

// ErrorFrame creates a BaseFrame containing an error response
func ErrorFrame(from string, seq uint64, err *Error) *BaseFrame {
	return NewBaseFrame(0, from, seq, err) // Kind 0 reserved for errors
}

// IsErrorFrame checks if a frame contains an error
func IsErrorFrame(frame *BaseFrame) bool {
	return frame.Kind == 0
}

// ExtractError extracts an Error from an error frame
func ExtractError(frame *BaseFrame) (*Error, error) {
	if !IsErrorFrame(frame) {
		return nil, fmt.Errorf("frame is not an error frame")
	}

	err, ok := frame.Body.(*Error)
	if !ok {
		return nil, fmt.Errorf("frame body is not an Error")
	}

	return err, nil
}
