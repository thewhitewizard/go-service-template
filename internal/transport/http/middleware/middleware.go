// Package middleware provides the HTTP middlewares of the service: request ID
// propagation, structured logging and metrics recording.
//
// It lives in the transport layer and is, together with the handlers, the only
// place allowed to know the web framework.
package middleware

import (
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

// localsKey types the keys stored in c.Locals, so a key added here can never
// collide with one added by a third-party middleware.
type localsKey string

const requestIDLocal localsKey = "request_id"

// requestIDHeader carries the request identifier in and out.
const requestIDHeader = "X-Request-ID"

// maxRequestIDLen bounds an inbound request identifier. Without it a caller can
// inject an arbitrarily long value that then lands in every log line of the
// request, which is a cheap way to flood a log pipeline.
const maxRequestIDLen = 128

// RequestID reads an inbound request identifier or generates one, stores it in
// the request Locals and echoes it in the response header.
func RequestID() fiber.Handler {
	return func(c fiber.Ctx) error {
		id := c.Get(requestIDHeader)
		if id == "" || len(id) > maxRequestIDLen {
			id = uuid.NewString()
		}

		c.Locals(requestIDLocal, id)
		c.Set(requestIDHeader, id)

		return c.Next()
	}
}

// RequestIDFromCtx returns the identifier stored by RequestID, or "" if the
// middleware did not run.
func RequestIDFromCtx(c fiber.Ctx) string {
	if id, ok := c.Locals(requestIDLocal).(string); ok {
		return id
	}

	return ""
}

// Logger logs every finished request as structured JSON through slog.
//
// It logs the request path, unlike the metrics middleware which labels by route
// pattern: a log line is one event and can afford the detail, whereas a metric
// label multiplies time series. The two are deliberately not symmetric.
func Logger(log *slog.Logger) fiber.Handler {
	return func(c fiber.Ctx) error {
		start := time.Now()

		err := c.Next()

		attrs := []slog.Attr{
			slog.String("method", c.Method()),
			slog.String("path", c.Path()),
			slog.Int("status", c.Response().StatusCode()),
			slog.Duration("duration", time.Since(start)),
			slog.String("request_id", RequestIDFromCtx(c)),
		}

		if err != nil {
			attrs = append(attrs, slog.String("error", err.Error()))
			log.LogAttrs(c.Context(), slog.LevelError, "http request", attrs...)

			return err
		}

		log.LogAttrs(c.Context(), slog.LevelInfo, "http request", attrs...)

		return nil
	}
}
