// Package domain holds the application's core types and error sentinels.
//
// It knows nothing about HTTP, about the web framework, or about any storage
// engine: everything here is expressed with context.Context and plain structs.
// That is what keeps the transport layer replaceable, and it is enforced by the
// architecture test in internal/arch.
package domain

import "errors"

// Sentinel errors that the transport layer maps to HTTP status codes. Callers
// compare with errors.Is, never by matching the message: a message is for
// humans, a sentinel is for code.
//
// Wrap them with fmt.Errorf("...: %w", err) to add context while keeping the
// sentinel reachable. Never wrap a value that carries a secret (a DSN, a token,
// a signed URL) into an error that will reach the logs.
var (
	// ErrNotFound reports that a requested resource does not exist. → 404.
	ErrNotFound = errors.New("resource not found")

	// ErrInvalidInput reports caller-supplied data that fails validation. → 400.
	ErrInvalidInput = errors.New("invalid input")

	// ErrConflict reports a uniqueness or state conflict. → 409.
	ErrConflict = errors.New("resource conflict")

	// ErrUnauthorized reports missing or invalid credentials. → 401
	//
	// Keep the message deliberately vague: telling a caller *why* authentication
	// failed (unknown subject vs. wrong secret) turns the endpoint into an oracle.
	ErrUnauthorized = errors.New("unauthorized")

	// ErrUnavailable reports that a dependency this service needs is unreachable. → 503.
	ErrUnavailable = errors.New("dependency unavailable")
)
