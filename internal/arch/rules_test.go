package arch_test

import (
	"path/filepath"
	"testing"
)

// This file is the negative test of the rule engine, and it is the reason the
// architecture test can be trusted.
//
// A guardrail introduced before the code it protects passes green without having
// verified anything: if matchesImportPrefix or isAllowed were subtly wrong — a
// missing separator, a prefix that swallows a neighbouring module — every rule in
// arch_test.go would report success forever and the boundary would be gone
// without a single failing test.
//
// So each primitive is tested on cases that MUST return false.

func TestMatchesImportPrefix(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		importPath string
		prefix     string
		want       bool
	}{
		{
			name:       "exact match",
			importPath: "github.com/gofiber",
			prefix:     "github.com/gofiber",
			want:       true,
		},
		{
			name:       "sub-package matches",
			importPath: "github.com/gofiber/fiber/v3/middleware/adaptor",
			prefix:     "github.com/gofiber",
			want:       true,
		},
		{
			// The case the "/" boundary exists for: without it, any module whose
			// name merely starts with the prefix would be caught, and a legitimate
			// import would be reported as an architecture violation.
			name:       "neighbouring module must NOT match",
			importPath: "github.com/gofiber-contrib/otelfiber",
			prefix:     "github.com/gofiber",
			want:       false,
		},
		{
			name:       "prometheus neighbour must NOT match",
			importPath: "github.com/prometheus-community/pro-bing",
			prefix:     prometheusImportPrefix,
			want:       false,
		},
		{
			name:       "shorter path must NOT match",
			importPath: "github.com/gofib",
			prefix:     "github.com/gofiber",
			want:       false,
		},
		{
			name:       "unrelated path",
			importPath: "log/slog",
			prefix:     "github.com/gofiber",
			want:       false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := matchesImportPrefix(tc.importPath, tc.prefix); got != tc.want {
				t.Fatalf("matchesImportPrefix(%q, %q) = %v, want %v",
					tc.importPath, tc.prefix, got, tc.want)
			}
		})
	}
}

func TestIsAllowed(t *testing.T) {
	t.Parallel()

	transport := filepath.Join("internal", "transport", "http")
	observability := filepath.Join("internal", "observability")

	tests := []struct {
		name    string
		rel     string
		allowed []string
		want    bool
	}{
		{
			name:    "file directly in an allowed directory",
			rel:     filepath.Join("internal", "transport", "http", "server.go"),
			allowed: []string{transport},
			want:    true,
		},
		{
			name:    "file in a sub-directory of an allowed directory",
			rel:     filepath.Join("internal", "transport", "http", "handlers", "health.go"),
			allowed: []string{transport},
			want:    true,
		},
		{
			// The case the separator exists for: a hand-written sibling package
			// whose name starts with the allowed directory must not inherit the
			// exemption.
			name:    "sibling directory with a common prefix must NOT be allowed",
			rel:     filepath.Join("internal", "observability-utils", "helper.go"),
			allowed: []string{observability},
			want:    false,
		},
		{
			name:    "parent directory must NOT be allowed",
			rel:     filepath.Join("internal", "transport", "registry.go"),
			allowed: []string{transport},
			want:    false,
		},
		{
			name:    "unrelated directory",
			rel:     filepath.Join("internal", "domain", "errors.go"),
			allowed: []string{transport, observability},
			want:    false,
		},
		{
			name:    "no allowed directory allows nothing",
			rel:     filepath.Join("internal", "domain", "errors.go"),
			allowed: nil,
			want:    false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := isAllowed(tc.rel, tc.allowed); got != tc.want {
				t.Fatalf("isAllowed(%q, %v) = %v, want %v", tc.rel, tc.allowed, got, tc.want)
			}
		})
	}
}
