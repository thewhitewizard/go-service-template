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

// StatusFor maps a domain error to an HTTP status code.
//
// This is the *only* translation point between the domain and HTTP. Keeping it
// here is what lets the domain stay free of the framework: a handler returns a
// domain sentinel, and this function decides what the client sees. Scattering
// fiber.NewError calls through the business code would put the mapping in a
// dozen places and guarantee they drift.
//
// An unknown error deliberately maps to 500 and its message is *not* forwarded:
// an error built deeper in the stack may quote a connection string, a token or a
// file path, and %w chains carry that all the way up.
func StatusFor(err error) int {
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
