package arch_test

import (
	"os"
	"path/filepath"
	"strings"
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

// TestImportRulePrefixesAreRealModules closes the one hole the tests below cannot
// see.
//
// A rule whose importPrefix contains a typo — "github.com/gofibre" instead of
// "github.com/gofiber" — matches nothing, finds no violation, and passes. The rule
// is *vacuously true*: the whole suite stays green while the boundary it claims to
// guard is wide open, and nothing anywhere turns red. Sharing the constant with
// this file does not help either, since both sides then use the same wrong value.
//
// The only thing that can catch it is an independent source of truth for what the
// dependency is actually called. go.mod is that source.
func TestImportRulePrefixesAreRealModules(t *testing.T) {
	t.Parallel()

	modules := modulePaths(t)

	for _, rule := range importRules {
		t.Run(rule.name, func(t *testing.T) {
			t.Parallel()

			for _, mod := range modules {
				// The prefix may be the module itself, or a parent namespace of it:
				// "github.com/prometheus" covers "github.com/prometheus/client_golang".
				if mod == rule.importPrefix || strings.HasPrefix(mod, rule.importPrefix+"/") {
					return
				}
			}

			t.Fatalf("rule %q guards import prefix %q, which matches no module in go.mod: "+
				"the rule can never fire and the boundary is unguarded — check for a typo",
				rule.name, rule.importPrefix)
		})
	}
}

// modulePaths returns every module path mentioned in go.mod, direct and indirect.
//
// Read as text rather than through `go list -m`: a test must not depend on the
// module cache or on the network to tell whether a rule is well spelled.
func modulePaths(t *testing.T) []string {
	t.Helper()

	raw, err := os.ReadFile(filepath.Join(moduleRoot(t), "go.mod"))
	if err != nil {
		t.Fatalf("reading go.mod: %v", err)
	}

	var paths []string

	for line := range strings.Lines(string(raw)) {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		// A require entry is "<module path> <version>". Filter on the version,
		// which is the only reliable marker: it skips the "module", "go",
		// "require" and "toolchain" directives without hardcoding them.
		if strings.HasPrefix(fields[1], "v") && strings.Contains(fields[0], "/") {
			paths = append(paths, fields[0])
		}
	}

	if len(paths) == 0 {
		t.Fatal("no module path found in go.mod — the parser is wrong, not the file")
	}

	return paths
}

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
			importPath: gofiberImportPrefix,
			prefix:     gofiberImportPrefix,
			want:       true,
		},
		{
			name:       "sub-package matches",
			importPath: gofiberImportPrefix + "/fiber/v3/middleware/adaptor",
			prefix:     gofiberImportPrefix,
			want:       true,
		},
		{
			// The case the "/" boundary exists for: without it, any module whose
			// name merely starts with the prefix would be caught, and a legitimate
			// import would be reported as an architecture violation.
			name:       "neighbouring module must NOT match",
			importPath: gofiberImportPrefix + "-contrib/otelfiber",
			prefix:     gofiberImportPrefix,
			want:       false,
		},
		{
			name:       "prometheus neighbour must NOT match",
			importPath: prometheusImportPrefix + "-community/pro-bing",
			prefix:     prometheusImportPrefix,
			want:       false,
		},
		{
			// Truncating the prefix must not match either: HasPrefix is applied to
			// the import path, not the other way round.
			name:       "shorter path must NOT match",
			importPath: "github.com/gofib",
			prefix:     gofiberImportPrefix,
			want:       false,
		},
		{
			name:       "unrelated path",
			importPath: "log/slog",
			prefix:     gofiberImportPrefix,
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

	domainFile := filepath.Join(dirDomain, "errors.go")

	tests := []struct {
		name    string
		rel     string
		allowed []string
		want    bool
	}{
		{
			name:    "file directly in an allowed directory",
			rel:     filepath.Join(dirTransportHTTP, "server.go"),
			allowed: []string{dirTransportHTTP},
			want:    true,
		},
		{
			name:    "file in a sub-directory of an allowed directory",
			rel:     filepath.Join(dirTransportHTTP, "handlers", "health.go"),
			allowed: []string{dirTransportHTTP},
			want:    true,
		},
		{
			// The case the separator exists for: a hand-written sibling package
			// whose name starts with the allowed directory must not inherit the
			// exemption.
			name:    "sibling directory with a common prefix must NOT be allowed",
			rel:     filepath.Join(dirInternal, "observability-utils", "helper.go"),
			allowed: []string{dirObservability},
			want:    false,
		},
		{
			name:    "parent directory must NOT be allowed",
			rel:     filepath.Join(dirInternal, "transport", "registry.go"),
			allowed: []string{dirTransportHTTP},
			want:    false,
		},
		{
			name:    "unrelated directory",
			rel:     domainFile,
			allowed: []string{dirTransportHTTP, dirObservability},
			want:    false,
		},
		{
			name:    "no allowed directory allows nothing",
			rel:     domainFile,
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
