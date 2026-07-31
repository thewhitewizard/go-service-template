package config

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeLookup builds a LookupFunc from a map, so no test mutates the process
// environment — which would make the suite order-dependent.
func fakeLookup(env map[string]string) LookupFunc {
	return func(key string) (string, bool) {
		v, ok := env[key]

		return v, ok
	}
}

func TestLoadDefaults(t *testing.T) {
	t.Parallel()

	cfg, err := load(fakeLookup(nil))
	require.NoError(t, err)

	assert.Equal(t, ":8080", cfg.ListenAddr)
	assert.Equal(t, "info", cfg.LogLevel)
	assert.Equal(t, 15*time.Second, cfg.ReadTimeout)
	assert.Equal(t, 120*time.Second, cfg.IdleTimeout)
	assert.Equal(t, 30*time.Second, cfg.WriteTimeout)
	assert.Equal(t, 20*time.Second, cfg.ShutdownTimeout)
}

func TestLoadDurationSpellings(t *testing.T) {
	t.Parallel()

	tests := map[string]time.Duration{
		"45s": 45 * time.Second,
		"2m":  2 * time.Minute,
		"90":  90 * time.Second, // plain integer of seconds, as written in manifests
		"0":   0,
	}

	for raw, want := range tests {
		t.Run(raw, func(t *testing.T) {
			t.Parallel()

			cfg, err := load(fakeLookup(map[string]string{EnvReadTimeout: raw}))
			require.NoError(t, err)
			assert.Equal(t, want, cfg.ReadTimeout)
		})
	}
}

// TestLoadTreatsBlankAsUnset pins the trimming rule. A value of " " reaches a
// process easily — a YAML manifest with a trailing space, an `ENV FOO= ` line, a
// copy-paste — and it must behave like "not set", not like a value.
//
// Without the trim, a blank listen address passed the emptiness check and only
// failed later, when the network stack refused to bind " ": an error message far
// from the variable that caused it.
func TestLoadTreatsBlankAsUnset(t *testing.T) {
	t.Parallel()

	cfg, err := load(fakeLookup(map[string]string{
		EnvListenAddr:  "   ",
		EnvLogLevel:    "\t",
		EnvReadTimeout: " ",
	}))
	require.NoError(t, err)

	assert.Equal(t, ":8080", cfg.ListenAddr)
	assert.Equal(t, "info", cfg.LogLevel)
	assert.Equal(t, 15*time.Second, cfg.ReadTimeout)
}

// TestLoadTrimsSurroundingWhitespace covers the other half: a real value wrapped
// in whitespace is the value, not an error.
func TestLoadTrimsSurroundingWhitespace(t *testing.T) {
	t.Parallel()

	cfg, err := load(fakeLookup(map[string]string{
		EnvListenAddr:  " :9090 ",
		EnvLogLevel:    " debug ",
		EnvReadTimeout: " 45s ",
	}))
	require.NoError(t, err)

	assert.Equal(t, ":9090", cfg.ListenAddr)
	assert.Equal(t, "debug", cfg.LogLevel)
	assert.Equal(t, 45*time.Second, cfg.ReadTimeout)
}

// TestValidateRejectsEmptyListenAddr guards the validation itself, independently
// of the environment: load always supplies a default, so only a Config built in
// code can reach validate with an empty address — and it must still be refused.
func TestValidateRejectsEmptyListenAddr(t *testing.T) {
	t.Parallel()

	err := Config{LogLevel: "info"}.validate()

	require.Error(t, err)
	assert.Contains(t, err.Error(), EnvListenAddr)
}

// TestLoadRejectsInvalidValues checks that a bad value stops the launch. Each case
// must fail, and the message must name the offending variable — otherwise an
// operator gets "invalid config" and no way to find which knob is wrong.
func TestLoadRejectsInvalidValues(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		env      map[string]string
		wantName string
	}{
		"unknown log level": {
			env:      map[string]string{EnvLogLevel: "verbose"},
			wantName: EnvLogLevel,
		},
		"unparseable duration": {
			env:      map[string]string{EnvIdleTimeout: "soon"},
			wantName: EnvIdleTimeout,
		},
		"negative duration": {
			env:      map[string]string{EnvShutdownTimeout: "-5s"},
			wantName: EnvShutdownTimeout,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, err := load(fakeLookup(tc.env))
			require.Error(t, err, "an invalid value must fail the launch")
			assert.Contains(t, err.Error(), tc.wantName,
				"the error must name the offending variable")
		})
	}
}

// TestLoadIsFailFastOnFirstBadDuration pins the ordered resolution: the first
// invalid duration stops the load, so the reported error is the first problem and
// not whichever one the struct literal happened to evaluate last.
func TestLoadIsFailFastOnFirstBadDuration(t *testing.T) {
	t.Parallel()

	_, err := load(fakeLookup(map[string]string{
		EnvReadTimeout: "nope",
		EnvIdleTimeout: "also-nope",
	}))

	require.Error(t, err)
	assert.Contains(t, err.Error(), EnvReadTimeout)
	assert.NotContains(t, err.Error(), EnvIdleTimeout)
}

// TestLogValueDoesNotLeakUnlistedFields is the guard that keeps the redaction
// honest as the struct grows: LogValue must enumerate what it prints, so a field
// added later is invisible to logs until someone consciously lists it.
//
// When you add a secret, add it here too — as "[REDACTED]", never as its value.
func TestLogValueDoesNotLeakUnlistedFields(t *testing.T) {
	t.Parallel()

	cfg := Config{ListenAddr: ":9999", LogLevel: "debug", ReadTimeout: time.Second}

	var buf bytes.Buffer

	log := slog.New(slog.NewJSONHandler(&buf, nil))
	log.Info("startup", slog.Any("config", cfg))

	var entry map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &entry))

	group, ok := entry["config"].(map[string]any)
	require.True(t, ok, "config must be logged as a group")

	assert.Equal(t, ":9999", group["listen_addr"])
	assert.Equal(t, "debug", group["log_level"])

	// The set of logged keys is closed on purpose.
	expected := []string{
		"listen_addr", "log_level",
		"read_timeout", "idle_timeout", "write_timeout", "shutdown_timeout",
	}
	assert.Len(t, group, len(expected))

	for _, key := range expected {
		assert.Contains(t, group, key)
	}
}
