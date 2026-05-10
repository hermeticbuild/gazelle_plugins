package ts

import (
	"reflect"
	"testing"
)

func TestIsTypeScriptFile(t *testing.T) {
	cfg := newTsConfig()
	cases := map[string]bool{
		"a.ts":      true,
		"a.tsx":     true,
		"a.js":      false,
		"a.json":    false,
		"a.d.ts":    true,
		"a.test.ts": true,
	}
	for name, want := range cases {
		if got := isTypeScriptFile(name, cfg); got != want {
			t.Errorf("isTypeScriptFile(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestIsTypeScriptFile_CustomExtensions(t *testing.T) {
	cfg := newTsConfig()
	cfg.extensions = append(cfg.extensions, ".mts")
	if !isTypeScriptFile("foo.mts", cfg) {
		t.Errorf("expected .mts to be recognized after directive")
	}
}

func TestIsTestFile_DefaultPatterns(t *testing.T) {
	cfg := newTsConfig()
	cases := map[string]bool{
		"foo.test.ts":                true,
		"foo.test.tsx":               true,
		"tests/index.ts":             true,
		"test/main.ts":               true,
		"src/foo.ts":                 false,
		"foo.spec.ts":                false, // not in default patterns
		"deeply/nested/test/file.ts": false, // patterns are top-level prefixes
	}
	for name, want := range cases {
		if got := isTestFile(name, cfg); got != want {
			t.Errorf("isTestFile(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestIsTestFile_CustomPatterns(t *testing.T) {
	cfg := newTsConfig()
	cfg.testPatterns = append(cfg.testPatterns, "*.spec.ts")
	if !isTestFile("foo.spec.ts", cfg) {
		t.Errorf("custom *.spec.ts pattern not picked up")
	}
}

func TestMatchTestPattern(t *testing.T) {
	cases := []struct {
		pattern string
		name    string
		want    bool
	}{
		{"*.test.ts", "foo.test.ts", true},
		{"*.test.ts", "foo.ts", false},
		{"tests/**", "tests/foo.ts", true},
		{"tests/**", "tests/sub/foo.ts", true},
		{"tests/**", "src/tests/foo.ts", false},
		{"foo.ts", "foo.ts", true},
		{"foo.ts", "bar.ts", false},
	}
	for _, c := range cases {
		got := matchTestPattern(c.pattern, c.name)
		if got != c.want {
			t.Errorf("matchTestPattern(%q, %q) = %v, want %v", c.pattern, c.name, got, c.want)
		}
	}
}

func TestResolveRuleNames(t *testing.T) {
	cases := []struct {
		name     string
		cfg      *tsConfig
		rel      string
		wantLib  string
		wantTest string
	}{
		{
			name:     "default uses package basename",
			cfg:      newTsConfig(),
			rel:      "apps/web",
			wantLib:  "web",
			wantTest: "web_test",
		},
		{
			name:     "deeply nested uses leaf basename",
			cfg:      newTsConfig(),
			rel:      "packages/utils/math/deep",
			wantLib:  "deep",
			wantTest: "deep_test",
		},
		{
			name:     "repo root falls back to literal lib/test",
			cfg:      newTsConfig(),
			rel:      "",
			wantLib:  "lib",
			wantTest: "test",
		},
		{
			name: "directive overrides win",
			cfg: func() *tsConfig {
				c := newTsConfig()
				c.libraryName = "src"
				c.testName = "spec"
				return c
			}(),
			rel:      "packages/foo",
			wantLib:  "src",
			wantTest: "spec",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			lib, test := resolveRuleNames(c.cfg, c.rel)
			if lib != c.wantLib {
				t.Errorf("lib = %q, want %q", lib, c.wantLib)
			}
			if test != c.wantTest {
				t.Errorf("test = %q, want %q", test, c.wantTest)
			}
		})
	}
}

func TestCollectSrcs(t *testing.T) {
	cfg := newTsConfig()
	files := []string{
		"main.ts",
		"helper.ts",
		"types.tsx",
		"main.test.ts",
		"tests/integration.ts",
		"README.md",
		"package.json",
	}
	parts := collectSrcs(files, cfg)

	wantLibs := []string{"helper.ts", "main.ts", "types.tsx"}
	wantTests := []string{"main.test.ts", "tests/integration.ts"}
	if !reflect.DeepEqual(parts.lib, wantLibs) {
		t.Errorf("libs = %v, want %v", parts.lib, wantLibs)
	}
	if !reflect.DeepEqual(parts.test, wantTests) {
		t.Errorf("tests = %v, want %v", parts.test, wantTests)
	}
	if len(parts.bundlerConfigs) != 0 {
		t.Errorf("bundlerConfigs = %v, want empty", parts.bundlerConfigs)
	}
}

func TestCollectSrcs_BundlerConfigSplit(t *testing.T) {
	cfg := newTsConfig()
	cfg.bundlerConfigSpecs = []bundlerConfigSpec{
		{Pattern: "vite.config.*", Name: "vite_config"},
		{Pattern: "vitest.config.*", Name: "vitest_config"},
	}
	files := []string{
		"index.ts",
		"vite.config.ts",
		"vitest.config.ts",
		"index.test.ts",
		"helper.ts",
	}
	parts := collectSrcs(files, cfg)

	wantLibs := []string{"helper.ts", "index.ts"}
	if !reflect.DeepEqual(parts.lib, wantLibs) {
		t.Errorf("lib = %v, want %v", parts.lib, wantLibs)
	}
	wantTests := []string{"index.test.ts"}
	if !reflect.DeepEqual(parts.test, wantTests) {
		t.Errorf("test = %v, want %v", parts.test, wantTests)
	}
	if got := parts.bundlerConfigs[0]; !reflect.DeepEqual(got, []string{"vite.config.ts"}) {
		t.Errorf("vite bucket = %v, want [vite.config.ts]", got)
	}
	if got := parts.bundlerConfigs[1]; !reflect.DeepEqual(got, []string{"vitest.config.ts"}) {
		t.Errorf("vitest bucket = %v, want [vitest.config.ts]", got)
	}
}

func TestMatchBundlerConfigSpec_LongestPatternWins(t *testing.T) {
	cfg := newTsConfig()
	cfg.bundlerConfigSpecs = []bundlerConfigSpec{
		// Order is intentional — the more-specific pattern is declared after
		// the broader one, and longest-pattern-wins must override declaration
		// order.
		{Pattern: "vite.config.*", Name: "vite_config"},
		{Pattern: "vite.config.production.ts", Name: "vite_prod_config"},
	}
	idx, ok := matchBundlerConfigSpec("vite.config.production.ts", cfg)
	if !ok || idx != 1 {
		t.Errorf("longest-pattern-wins failed: idx=%d ok=%v", idx, ok)
	}
	idx, ok = matchBundlerConfigSpec("vite.config.ts", cfg)
	if !ok || idx != 0 {
		t.Errorf("broader pattern should match: idx=%d ok=%v", idx, ok)
	}
	if _, ok := matchBundlerConfigSpec("nope.ts", cfg); ok {
		t.Errorf("non-matching file matched")
	}
}

func TestCollectSrcs_BundlerOverridesTest(t *testing.T) {
	// A file matching both a test pattern and a bundler-config pattern goes
	// to the bundler-config bucket — the boundary the directive enforces is
	// stronger than the test split.
	cfg := newTsConfig()
	cfg.testPatterns = append(cfg.testPatterns, "*.config.ts")
	cfg.bundlerConfigSpecs = []bundlerConfigSpec{
		{Pattern: "vite.config.ts", Name: "vite_config"},
	}
	parts := collectSrcs([]string{"vite.config.ts", "index.ts"}, cfg)

	if len(parts.test) != 0 {
		t.Errorf("test bucket should be empty, got %v", parts.test)
	}
	if got := parts.bundlerConfigs[0]; !reflect.DeepEqual(got, []string{"vite.config.ts"}) {
		t.Errorf("bundler bucket = %v, want [vite.config.ts]", got)
	}
}

func TestManagedBinaryKinds_IncludesBoth(t *testing.T) {
	// ts_binary and js_binary should both flow through the same data-attr
	// management path. Drift here means the discovery loop in GenerateRules
	// silently skips one of them.
	want := map[string]bool{KindJsBinary: true, KindTsBinary: true}
	got := map[string]bool{}
	for _, k := range managedBinaryKinds {
		got[k] = true
	}
	if !reflect.DeepEqual(want, got) {
		t.Errorf("managedBinaryKinds = %v, want %v", got, want)
	}
}

func TestKinds_HasTsBinary(t *testing.T) {
	// Without a KindInfo entry the merge engine treats ts_binary rules as
	// unmanaged: data wouldn't be a ResolveAttr and wouldn't be replaced.
	if _, ok := tsKinds[KindTsBinary]; !ok {
		t.Fatalf("tsKinds missing %q", KindTsBinary)
	}
	info := tsKinds[KindTsBinary]
	if !info.ResolveAttrs["data"] {
		t.Errorf("ts_binary should have data as ResolveAttr")
	}
	if !info.MergeableAttrs["data"] {
		t.Errorf("ts_binary should have data as MergeableAttr")
	}
}

func TestKinds_TsconfigTypesMergeable(t *testing.T) {
	for _, kind := range []string{KindTsLibrary, KindTsTest, KindTsBinary, KindBundlerConfig} {
		info := tsKinds[kind]
		if !info.ResolveAttrs["tsconfig_types"] {
			t.Errorf("%s should have tsconfig_types as ResolveAttr", kind)
		}
		if !info.MergeableAttrs["tsconfig_types"] {
			t.Errorf("%s should have tsconfig_types as MergeableAttr", kind)
		}
	}
}

func TestMatchBundlerConfigSpec_DoubleStar(t *testing.T) {
	cfg := newTsConfig()
	cfg.bundlerConfigSpecs = []bundlerConfigSpec{
		{Pattern: "**/main.ts", Name: "storybook_config"},
	}
	if _, ok := matchBundlerConfigSpec(".storybook/main.ts", cfg); !ok {
		t.Errorf("**/main.ts should match .storybook/main.ts")
	}
	if _, ok := matchBundlerConfigSpec("src/main.ts", cfg); !ok {
		t.Errorf("**/main.ts should match src/main.ts")
	}
}
