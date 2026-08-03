// Package handlers holds the HTTP handlers of the service and the single place
// where domain errors become HTTP status codes.
package handlers

import (
	"errors"

	"github.com/gofiber/fiber/v3"

	"github.com/thewhitewizard/go-service-template/internal/domain"
)

// Route paths, declared once so handlers, tests and the router agree.
const (
	PathHealthz = "/healthz"
	PathReadyz  = "/readyz"
	PathMetrics = "/metrics"
)

// statusKey is the JSON field every status response shares.
const statusKey = "status"

// StatusFor maps an error to the HTTP status code the client will receive.
//
// This is the *only* translation point between the domain and HTTP, and it has
// two consumers that must never disagree: the server's ErrorHandler, which
// writes the response, and the metrics middleware, which records what was sent.
// Computing the status in two places is how a dashboard ends up reporting 500s
// for requests the client saw as 404s.
//
// A *fiber.Error carries a status chosen deliberately by the transport layer, so
// it wins. Everything else is matched against the domain sentinels, and an
// unrecognised error maps to 500 — its message is never forwarded to the client,
// because an error built deeper in the stack may quote a connection string, a
// token or a file path, and %w chains carry that all the way up.
func StatusFor(err error) int {
	var fiberErr *fiber.Error
	if errors.As(err, &fiberErr) {
		return fiberErr.Code
	}

	switch {
	case err == nil:
		return fiber.StatusOK
	case errors.Is(err, domain.ErrInvalidInput):
		return fiber.StatusBadRequest
	case errors.Is(err, domain.ErrUnauthorized):
		return fiber.StatusUnauthorized
	case errors.Is(err, domain.ErrNotFound):
		return fiber.StatusNotFound
	case errors.Is(err, domain.ErrConflict):
		return fiber.StatusConflict
	case errors.Is(err, domain.ErrUnavailable):
		return fiber.StatusServiceUnavailable
	default:
		return fiber.StatusInternalServerError
	}
}
