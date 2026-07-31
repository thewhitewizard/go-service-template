// Package arch_test holds the architecture tests: a table of forbidden imports,
// each bounded to the set of directories allowed to use it.
//
// These rules make the layer boundaries claimed in CLAUDE.md and in docs/adr/
// actually enforceable. A boundary that only exists in prose is a boundary that
// has already been crossed somewhere.
package arch_test

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Import prefixes and layer directories, declared once and shared with the unit
// tests of the rule engine (rules_test.go).
//
// Note what sharing them does NOT buy: it does not protect against a typo in a
// prefix. Both the rule and the test would then use the same wrong value and stay
// green — and a rule whose prefix matches nothing finds no violation, so it passes
// vacuously while the boundary it claims to guard is wide open. That hole is closed
// by TestImportRulePrefixesAreRealModules in rules_test.go, which checks each
// prefix against the module list in go.mod.
const (
	gofiberImportPrefix    = "github.com/gofiber"
	fasthttpImportPrefix   = "github.com/valyala/fasthttp"
	prometheusImportPrefix = "github.com/prometheus"

	dirInternal = "internal"
)

// Layer directories, relative to the module root. Not constants because
// filepath.Join is a function call, and hardcoding a separator would break the
// architecture test on Windows.
var (
	dirTransportHTTP = filepath.Join(dirInternal, "transport", "http")
	dirObservability = filepath.Join(dirInternal, "observability")
	dirDomain        = filepath.Join(dirInternal, "domain")
)

// importRule describes one architecture constraint: importPrefix may only appear
// in files located under one of allowedDirs (relative to the module root).
//
// Rules apply to ALL .go files, tests included. A test that imports the web
// framework from the domain crosses the boundary just as effectively as
// production code, and the exemption would be invisible in review. A legitimate
// exception is therefore declared explicitly, by adding its directory to
// allowedDirs.
type importRule struct {
	name         string
	importPrefix string
	allowedDirs  []string
}

// importRules encodes the layer boundaries of this service. See docs/adr/ for the
// justification of each one.
var importRules = []importRule{
	{
		// ADR-0001: Fiber lives only in the HTTP transport layer, tests included,
		// so switching to net/http stays possible without touching the domain.
		// The prefix covers the whole gofiber ecosystem (utils, schema, …), not
		// just the fiber module itself.
		name:         "FiberStaysInTransportLayer",
		importPrefix: gofiberImportPrefix,
		allowedDirs:  []string{dirTransportHTTP},
	},
	{
		// fasthttp is Fiber's transitive dependency: importing it directly would
		// bypass the rule above and reopen the memory-reuse traps outside the
		// layer that knows how to handle them. Covering only the top-level
		// dependency of a framework leaves the boundary trivially escapable.
		name:         "FasthttpStaysInTransportLayer",
		importPrefix: fasthttpImportPrefix,
		allowedDirs:  []string{dirTransportHTTP},
	},
	{
		// ADR-0002: the metrics implementation lives in internal/observability,
		// which exposes narrow methods instead of prometheus types. The prefix is
		// the whole github.com/prometheus namespace, not just client_golang:
		// client_model and common would otherwise be a side door to the same
		// coupling — the same lesson as fasthttp above.
		name:         "PrometheusStaysInObservability",
		importPrefix: prometheusImportPrefix,
		allowedDirs:  []string{dirObservability},
	},
}

func TestImportRules(t *testing.T) {
	t.Parallel()

	for _, rule := range importRules {
		t.Run(rule.name, func(t *testing.T) {
			t.Parallel()
			checkImportRule(t, rule)
		})
	}
}

// checkImportRule walks the module tree and fails if a file outside the
// directories allowed by the rule imports the forbidden prefix.
func checkImportRule(t *testing.T, rule importRule) {
	t.Helper()

	root := moduleRoot(t)

	var violations []string

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			if skipDir(d.Name()) {
				return filepath.SkipDir
			}

			return nil
		}

		if !strings.HasSuffix(path, ".go") {
			return nil
		}

		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}

		if isAllowed(rel, rule.allowedDirs) {
			return nil
		}

		if importsPrefix(t, path, rule.importPrefix) {
			violations = append(violations, rel)
		}

		return nil
	})
	if err != nil {
		t.Fatalf("walking the module tree: %v", err)
	}

	if len(violations) > 0 {
		t.Fatalf("%s imported outside the directories allowed by rule %q:\n  %s",
			rule.importPrefix, rule.name, strings.Join(violations, "\n  "))
	}
}

// importsPrefix reports whether the file at path imports prefix, either exactly
// or as an import path prefix (a sub-package).
func importsPrefix(t *testing.T, path, prefix string) bool {
	t.Helper()

	fset := token.NewFileSet()

	file, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}

	for _, imp := range file.Imports {
		if matchesImportPrefix(strings.Trim(imp.Path.Value, `"`), prefix) {
			return true
		}
	}

	return false
}

// matchesImportPrefix reports whether importPath is prefix itself or one of its
// sub-packages. The "/" boundary avoids catching a neighbouring module whose name
// merely starts with the prefix.
func matchesImportPrefix(importPath, prefix string) bool {
	return importPath == prefix || strings.HasPrefix(importPath, prefix+"/")
}

// isAllowed reports whether the file rel sits in one of the allowed directories.
func isAllowed(rel string, allowedDirs []string) bool {
	for _, dir := range allowedDirs {
		if rel == dir || strings.HasPrefix(rel, dir+string(os.PathSeparator)) {
			return true
		}
	}

	return false
}

func skipDir(name string) bool {
	switch name {
	// testdata may hold deliberately invalid .go files (a Go convention): parsing
	// them would fail the architecture test for an unrelated reason.
	case ".git", "vendor", "bin", "dist", "node_modules", "testdata":
		return true
	default:
		return false
	}
}

// moduleRoot walks up from this test file to the directory holding go.mod.
func moduleRoot(t *testing.T) string {
	t.Helper()

	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate the test file")
	}

	dir := filepath.Dir(thisFile)

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found walking up from the test")
		}

		dir = parent
	}
}
