// Package config loads and validates the service configuration from the
// environment.
//
// Configuration is resolved once, at startup: an invalid value fails the launch
// instead of silently degrading the service at request time. A value read on the
// fly inside a handler is a bug — it cannot be validated, and it makes the
// running behaviour depend on when the process happened to read it.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

// Environment variable names. Keeping them as exported constants means tests and
// documentation reference the same string the loader reads.
const (
	EnvListenAddr      = "SERVICE_LISTEN_ADDR"
	EnvReadTimeout     = "SERVICE_READ_TIMEOUT"
	EnvIdleTimeout     = "SERVICE_IDLE_TIMEOUT"
	EnvWriteTimeout    = "SERVICE_WRITE_TIMEOUT"
	EnvShutdownTimeout = "SERVICE_SHUTDOWN_TIMEOUT"
	EnvLogLevel        = "SERVICE_LOG_LEVEL"
)

// LookupFunc resolves an environment variable. It has the signature of
// os.LookupEnv; injecting it makes Load testable without mutating the process
// environment, which would make tests order-dependent.
type LookupFunc func(key string) (string, bool)

// Config holds every runtime parameter of the service.
type Config struct {
	// ListenAddr is the HTTP listen address (for example ":8080").
	ListenAddr string
	// ReadTimeout bounds reading the incoming request.
	ReadTimeout time.Duration
	// IdleTimeout bounds idle keep-alive connections.
	IdleTimeout time.Duration
	// WriteTimeout bounds writing the response. Keep it at 0 (unlimited) if this
	// service ever streams a long response, otherwise the stream is cut mid-flight.
	WriteTimeout time.Duration
	// ShutdownTimeout bounds graceful shutdown.
	ShutdownTimeout time.Duration
	// LogLevel is one of "debug", "info", "warn", "error".
	LogLevel string
}

// LogValue controls what a logger is allowed to print for a Config.
//
// Without it, a slog.Any("config", cfg) added one day to a startup or debug log
// would write every field in clear text — including any secret added later — and
// logs are not retroactively redactable. Implementing it now means the redaction
// exists before the first secret does.
//
// When you add a secret field, add it here as "[REDACTED]", and reduce any
// connection string to the parts that help diagnose a misconfiguration (scheme,
// host, database name) and never the credentials. For example:
//
//	slog.String("api_token", "[REDACTED]"),
//	slog.String("database", redactDSN(c.DatabaseURL)),
func (c Config) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("listen_addr", c.ListenAddr),
		slog.String("log_level", c.LogLevel),
		slog.Duration("read_timeout", c.ReadTimeout),
		slog.Duration("idle_timeout", c.IdleTimeout),
		slog.Duration("write_timeout", c.WriteTimeout),
		slog.Duration("shutdown_timeout", c.ShutdownTimeout),
	)
}

// Load reads the configuration from the process environment and validates it.
func Load() (Config, error) {
	return load(os.LookupEnv)
}

// load resolves the configuration through lookup and validates it. Split from
// Load so tests can supply a fake lookup.
func load(lookup LookupFunc) (Config, error) {
	cfg := Config{
		ListenAddr: getEnv(lookup, EnvListenAddr, ":8080"),
		LogLevel:   getEnv(lookup, EnvLogLevel, "info"),
	}

	// Durations are resolved through an explicit, ordered sequence: the first
	// invalid value stops the load (fail fast), without relying on the evaluation
	// order of struct literal fields.
	durations := []struct {
		key      string
		fallback time.Duration
		dst      *time.Duration
	}{
		{EnvReadTimeout, 15 * time.Second, &cfg.ReadTimeout},
		{EnvIdleTimeout, 120 * time.Second, &cfg.IdleTimeout},
		{EnvWriteTimeout, 30 * time.Second, &cfg.WriteTimeout},
		{EnvShutdownTimeout, 20 * time.Second, &cfg.ShutdownTimeout},
	}

	for _, d := range durations {
		v, err := getEnvDuration(lookup, d.key, d.fallback)
		if err != nil {
			return Config{}, err
		}

		*d.dst = v
	}

	if err := cfg.validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func (c Config) validate() error {
	if c.ListenAddr == "" {
		return errors.New("config: " + EnvListenAddr + " must not be empty")
	}

	switch c.LogLevel {
	case "debug", "info", "warn", "error":
	default:
		return fmt.Errorf("config: invalid %s %q (want debug|info|warn|error)", EnvLogLevel, c.LogLevel)
	}

	durations := []struct {
		name  string
		value time.Duration
	}{
		{EnvReadTimeout, c.ReadTimeout},
		{EnvIdleTimeout, c.IdleTimeout},
		{EnvWriteTimeout, c.WriteTimeout},
		{EnvShutdownTimeout, c.ShutdownTimeout},
	}

	for _, d := range durations {
		if d.value < 0 {
			return fmt.Errorf("config: %s must not be negative, got %s", d.name, d.value)
		}
	}

	return nil
}

// getEnv resolves key, falling back when it is unset or blank.
//
// The value is trimmed, and a whitespace-only value is treated as unset. This is
// not pedantry: a value of " " reaches a process easily (a YAML manifest with a
// trailing space, an `ENV FOO= ` line, a copy-paste) and without the trim it
// passes an emptiness check, then fails much later — a listen address of " " gets
// rejected by the network stack, far from the variable that caused it.
func getEnv(lookup LookupFunc, key, fallback string) string {
	if v, ok := lookup(key); ok {
		if trimmed := strings.TrimSpace(v); trimmed != "" {
			return trimmed
		}
	}

	return fallback
}

func getEnvDuration(lookup LookupFunc, key string, fallback time.Duration) (time.Duration, error) {
	raw, ok := lookup(key)
	if !ok {
		return fallback, nil
	}

	v := strings.TrimSpace(raw)
	if v == "" {
		return fallback, nil
	}

	// Accept either a Go duration ("30s", "2m") or a plain integer of seconds,
	// because both spellings show up in hand-written deployment manifests.
	d, durErr := time.ParseDuration(v)
	if durErr == nil {
		return d, nil
	}

	secs, atoiErr := strconv.Atoi(v)
	if atoiErr == nil {
		return time.Duration(secs) * time.Second, nil
	}

	return 0, fmt.Errorf("config: invalid %s %q (want a Go duration or an integer of seconds): %w",
		key, v, errors.Join(durErr, atoiErr))
}
