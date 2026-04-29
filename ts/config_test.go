package ts

import (
	"reflect"
	"testing"

	"github.com/bazelbuild/bazel-gazelle/config"
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

func TestApplyDirective_ConfigFile(t *testing.T) {
	cfg := newTsConfig()
	applyDirective(cfg, rule.Directive{Key: directiveConfigFile, Value: "vite.config.* vite_config_deps"})
	applyDirective(cfg, rule.Directive{Key: directiveConfigFile, Value: "tailwind.config.ts vite_config_deps"})

	if len(cfg.configFiles) != 2 {
		t.Fatalf("configFiles = %v, want 2 entries", cfg.configFiles)
	}
	if cfg.configFiles[0] != (configFileSpec{pattern: "vite.config.*", attr: "vite_config_deps"}) {
		t.Errorf("entry 0 = %+v", cfg.configFiles[0])
	}
	if cfg.configFiles[1] != (configFileSpec{pattern: "tailwind.config.ts", attr: "vite_config_deps"}) {
		t.Errorf("entry 1 = %+v", cfg.configFiles[1])
	}

	// Malformed values are silently ignored — wrong field count.
	for _, bad := range []string{"", "only_one_field", "three  fields  here"} {
		before := len(cfg.configFiles)
		applyDirective(cfg, rule.Directive{Key: directiveConfigFile, Value: bad})
		if len(cfg.configFiles) != before {
			t.Errorf("malformed %q accepted; configFiles grew to %v", bad, cfg.configFiles)
		}
	}
}

func TestApplyDirective_GeneratedPackage(t *testing.T) {
	cfg := newTsConfig()
	applyDirective(cfg, rule.Directive{
		Key:   directiveGeneratedPackage,
		Value: "@myrepo_generated/*=//:node_modules/@myrepo_generated/*",
	})
	got := cfg.generatedPackages["@myrepo_generated/*"]
	if got != "//:node_modules/@myrepo_generated/*" {
		t.Errorf("generated package mapping = %q", got)
	}

	// Malformed (missing `=`) is silently ignored.
	applyDirective(cfg, rule.Directive{Key: directiveGeneratedPackage, Value: "noequals"})
	if _, ok := cfg.generatedPackages["noequals"]; ok {
		t.Errorf("malformed directive accepted")
	}
}

func TestClone_Independent(t *testing.T) {
	parent := newTsConfig()
	parent.libraryName = "lib"
	parent.testPatterns = append(parent.testPatterns, "*.spec.ts")
	parent.generatedPackages["#foo/*"] = "//foo"
	parent.configFiles = append(parent.configFiles, configFileSpec{pattern: "vite.config.*", attr: "vite_config_deps"})

	child := parent.clone()
	child.libraryName = "src"
	child.testPatterns = append(child.testPatterns, "**/__tests__/**")
	child.generatedPackages["#bar/*"] = "//bar"
	child.configFiles = append(child.configFiles, configFileSpec{pattern: "storybook.config.ts", attr: "storybook_deps"})

	if parent.libraryName != "lib" {
		t.Errorf("parent libraryName mutated: %q", parent.libraryName)
	}
	if contains(parent.testPatterns, "**/__tests__/**") {
		t.Errorf("parent testPatterns mutated: %v", parent.testPatterns)
	}
	if _, ok := parent.generatedPackages["#bar/*"]; ok {
		t.Errorf("parent generatedPackages mutated")
	}
	if len(parent.configFiles) != 1 {
		t.Errorf("parent configFiles mutated: %v", parent.configFiles)
	}
}

// Configure should register every ts_config_file attr as a ResolveAttr on
// both the default library kind and any directive-overridden libraryKind, so
// the merger replaces stale attr values across re-runs regardless of whether
// the user has switched to a custom kind.
func TestConfigure_RegistersConfigFileAttrsOnLibraryKinds(t *testing.T) {
	l := &tsLang{
		packageDeps:       make(map[string]bool),
		subpathImportsMap: make(map[string]string),
		kindInfos:         defaultKindInfos(),
	}
	c := config.New()
	c.Exts = map[string]interface{}{}

	f := &rule.File{Directives: []rule.Directive{
		{Key: directiveLibraryKind, Value: "custom_ts_project"},
		{Key: directiveConfigFile, Value: "vite.config.* vite_config_deps"},
		{Key: directiveConfigFile, Value: ".storybook/main.ts storybook_deps"},
	}}
	l.Configure(c, "apps/web", f)

	for _, kind := range []string{defaultLibraryKind, "custom_ts_project"} {
		info, ok := l.kindInfos[kind]
		if !ok {
			t.Errorf("kind %q not registered", kind)
			continue
		}
		for _, attr := range []string{"vite_config_deps", "storybook_deps"} {
			if !info.ResolveAttrs[attr] {
				t.Errorf("kind %q missing ResolveAttr %q: %v", kind, attr, info.ResolveAttrs)
			}
		}
	}

	// And the cfg the directives populated should be stashed in c.Exts for
	// downstream calls (GenerateRules, Resolve) to consume.
	cfg, ok := c.Exts[languageName].(*tsConfig)
	if !ok {
		t.Fatalf("c.Exts[%q] not *tsConfig: %T", languageName, c.Exts[languageName])
	}
	if len(cfg.configFiles) != 2 {
		t.Errorf("cfg.configFiles = %v, want 2 entries", cfg.configFiles)
	}
}

// When the user keeps the default libraryKind, Configure must NOT churn an
// extra synthetic kind — only the default one gets the registered attr.
func TestConfigure_SkipsSyntheticKindWhenDefault(t *testing.T) {
	l := &tsLang{
		packageDeps:       make(map[string]bool),
		subpathImportsMap: make(map[string]string),
		kindInfos:         defaultKindInfos(),
	}
	c := config.New()
	c.Exts = map[string]interface{}{}
	beforeKinds := len(l.kindInfos)

	f := &rule.File{Directives: []rule.Directive{
		{Key: directiveConfigFile, Value: "vite.config.* vite_config_deps"},
	}}
	l.Configure(c, "apps/web", f)

	if got := len(l.kindInfos); got != beforeKinds {
		t.Errorf("kindInfos size = %d, want %d (no synthetic kind for default libraryKind)", got, beforeKinds)
	}
	if !l.kindInfos[defaultLibraryKind].ResolveAttrs["vite_config_deps"] {
		t.Errorf("vite_config_deps not registered on default kind: %v", l.kindInfos[defaultLibraryKind].ResolveAttrs)
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
