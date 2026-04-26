package ts

import (
	"reflect"
	"testing"

	"github.com/bazelbuild/bazel-gazelle/rule"
)

func TestApplyDirective_Bools(t *testing.T) {
	cfg := newTsConfig()
	applyDirective(cfg, rule.Directive{Key: directiveEnabled, Value: "false"})
	if cfg.enabled {
		t.Fatalf("ts_enabled false: cfg.enabled = true")
	}
	applyDirective(cfg, rule.Directive{Key: directiveEnabled, Value: "true"})
	if !cfg.enabled {
		t.Fatalf("ts_enabled true: cfg.enabled = false")
	}
	applyDirective(cfg, rule.Directive{Key: directiveProjectRefs, Value: "no"})
	if cfg.projectReferences {
		t.Fatalf("ts_project_references no: still true")
	}
}

func TestApplyDirective_Strings(t *testing.T) {
	cfg := newTsConfig()
	applyDirective(cfg, rule.Directive{Key: directiveLibraryName, Value: "src"})
	applyDirective(cfg, rule.Directive{Key: directiveTestName, Value: "spec"})
	applyDirective(cfg, rule.Directive{Key: directiveLibraryKind, Value: "my_lib"})
	applyDirective(cfg, rule.Directive{Key: directiveTestKind, Value: "my_test"})
	applyDirective(cfg, rule.Directive{Key: directiveTsconfig, Value: "//:tsconfig"})
	applyDirective(cfg, rule.Directive{Key: directiveNpmLinkPattern, Value: "//pnpm:node_modules/{pkg}"})

	if cfg.libraryName != "src" {
		t.Errorf("libraryName = %q", cfg.libraryName)
	}
	if cfg.testName != "spec" {
		t.Errorf("testName = %q", cfg.testName)
	}
	if cfg.libraryKind != "my_lib" {
		t.Errorf("libraryKind = %q", cfg.libraryKind)
	}
	if cfg.testKind != "my_test" {
		t.Errorf("testKind = %q", cfg.testKind)
	}
	if cfg.tsconfig != "//:tsconfig" {
		t.Errorf("tsconfig = %q", cfg.tsconfig)
	}
	if cfg.npmLinkPattern != "//pnpm:node_modules/{pkg}" {
		t.Errorf("npmLinkPattern = %q", cfg.npmLinkPattern)
	}
}

func TestApplyDirective_Visibility(t *testing.T) {
	cfg := newTsConfig()
	applyDirective(cfg, rule.Directive{Key: directiveVisibility, Value: "//foo:__pkg__ //bar:__pkg__"})
	want := []string{"//foo:__pkg__", "//bar:__pkg__"}
	if !reflect.DeepEqual(cfg.visibility, want) {
		t.Errorf("visibility = %v want %v", cfg.visibility, want)
	}
}

func TestApplyDirective_AppendDirectives(t *testing.T) {
	cfg := newTsConfig()
	// Test patterns and extensions append rather than replace.
	applyDirective(cfg, rule.Directive{Key: directiveTestPattern, Value: "*.spec.ts"})
	applyDirective(cfg, rule.Directive{Key: directiveExtension, Value: ".mts"})
	applyDirective(cfg, rule.Directive{Key: directiveTestData, Value: "//:fixtures"})

	if !contains(cfg.testPatterns, "*.spec.ts") {
		t.Errorf("testPatterns missing *.spec.ts: %v", cfg.testPatterns)
	}
	if !contains(cfg.extensions, ".mts") {
		t.Errorf("extensions missing .mts: %v", cfg.extensions)
	}
	if !contains(cfg.testData, "//:fixtures") {
		t.Errorf("testData missing //:fixtures: %v", cfg.testData)
	}

	// Re-applying the same value should not duplicate.
	applyDirective(cfg, rule.Directive{Key: directiveTestPattern, Value: "*.spec.ts"})
	count := 0
	for _, p := range cfg.testPatterns {
		if p == "*.spec.ts" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected 1 occurrence of *.spec.ts, got %d", count)
	}
}

func TestApplyDirective_SubpathImport(t *testing.T) {
	cfg := newTsConfig()
	applyDirective(cfg, rule.Directive{
		Key:   directiveSubpathImport,
		Value: "@formatjs_generated/*=//:node_modules/@formatjs_generated/*",
	})
	got := cfg.subpathOverrides["@formatjs_generated/*"]
	if got != "//:node_modules/@formatjs_generated/*" {
		t.Errorf("subpath override = %q", got)
	}

	// Malformed (missing `=`) is silently ignored.
	applyDirective(cfg, rule.Directive{Key: directiveSubpathImport, Value: "noequals"})
	if _, ok := cfg.subpathOverrides["noequals"]; ok {
		t.Errorf("malformed directive accepted")
	}
}

func TestClone_Independent(t *testing.T) {
	parent := newTsConfig()
	parent.libraryName = "lib"
	parent.testPatterns = append(parent.testPatterns, "*.spec.ts")
	parent.subpathOverrides["#foo/*"] = "//foo"

	child := parent.clone()
	child.libraryName = "src"
	child.testPatterns = append(child.testPatterns, "**/__tests__/**")
	child.subpathOverrides["#bar/*"] = "//bar"

	if parent.libraryName != "lib" {
		t.Errorf("parent libraryName mutated: %q", parent.libraryName)
	}
	if contains(parent.testPatterns, "**/__tests__/**") {
		t.Errorf("parent testPatterns mutated: %v", parent.testPatterns)
	}
	if _, ok := parent.subpathOverrides["#bar/*"]; ok {
		t.Errorf("parent subpathOverrides mutated")
	}
}

func contains(slice []string, val string) bool {
	for _, s := range slice {
		if s == val {
			return true
		}
	}
	return false
}
