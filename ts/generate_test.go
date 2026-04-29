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
	libs, tests := collectSrcs(files, cfg, nil)

	wantLibs := []string{"helper.ts", "main.ts", "types.tsx"}
	wantTests := []string{"main.test.ts", "tests/integration.ts"}
	if !reflect.DeepEqual(libs, wantLibs) {
		t.Errorf("libs = %v, want %v", libs, wantLibs)
	}
	if !reflect.DeepEqual(tests, wantTests) {
		t.Errorf("tests = %v, want %v", tests, wantTests)
	}
}

func TestCollectSrcs_ExcludesConfigFiles(t *testing.T) {
	cfg := newTsConfig()
	files := []string{"main.ts", "vite.config.ts", "tailwind.config.ts", "main.test.ts"}
	matches := map[string]string{
		"vite.config.ts":     "vite_config_deps",
		"tailwind.config.ts": "vite_config_deps",
	}
	libs, tests := collectSrcs(files, cfg, matches)

	if !reflect.DeepEqual(libs, []string{"main.ts"}) {
		t.Errorf("libs = %v, want [main.ts] (config files excluded)", libs)
	}
	if !reflect.DeepEqual(tests, []string{"main.test.ts"}) {
		t.Errorf("tests = %v, want [main.test.ts]", tests)
	}
}

func TestMatchConfigFiles(t *testing.T) {
	cfg := newTsConfig()
	cfg.configFiles = []configFileSpec{
		{pattern: "vite.config.*", attr: "vite_config_deps"},
		{pattern: "tailwind.config.ts", attr: "tailwind_deps"},
	}
	files := []string{"main.ts", "vite.config.ts", "vite.config.production.ts", "tailwind.config.ts", "README.md"}

	got := matchConfigFiles(files, cfg)
	want := map[string]string{
		"vite.config.ts":            "vite_config_deps",
		"vite.config.production.ts": "vite_config_deps",
		"tailwind.config.ts":        "tailwind_deps",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("matchConfigFiles = %v, want %v", got, want)
	}
}

func TestMatchConfigFiles_LongestPatternWins(t *testing.T) {
	cfg := newTsConfig()
	// Two specs both match `vite.config.production.ts`; the longer / more
	// specific pattern claims it.
	cfg.configFiles = []configFileSpec{
		{pattern: "vite.config.*", attr: "default_bucket"},
		{pattern: "vite.config.production.*", attr: "prod_bucket"},
	}
	got := matchConfigFiles([]string{"vite.config.production.ts", "vite.config.ts"}, cfg)
	want := map[string]string{
		"vite.config.production.ts": "prod_bucket",
		"vite.config.ts":            "default_bucket",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("matchConfigFiles = %v, want %v", got, want)
	}
}

func TestMatchConfigFiles_EmptyConfig(t *testing.T) {
	cfg := newTsConfig()
	if got := matchConfigFiles([]string{"vite.config.ts"}, cfg); got != nil {
		t.Errorf("expected nil with no configFiles, got %v", got)
	}
}

// A file matched by a config pattern that *also* matches a test pattern
// (e.g. `vite.config.test.ts` vs `vite.config.*` and the default `*.test.ts`)
// must be claimed by the config bucket — never silently surfacing as a test
// is part of the directive's contract.
func TestCollectSrcs_ConfigFileBeatsTestPattern(t *testing.T) {
	cfg := newTsConfig()
	files := []string{"main.ts", "vite.config.test.ts", "main.test.ts"}
	matches := matchConfigFiles(append([]string(nil), files...), &tsConfig{
		extensions:   cfg.extensions,
		testPatterns: cfg.testPatterns,
		configFiles:  []configFileSpec{{pattern: "vite.config.*", attr: "vite_config_deps"}},
	})
	if matches["vite.config.test.ts"] != "vite_config_deps" {
		t.Fatalf("vite.config.test.ts not matched as config: %v", matches)
	}

	libs, tests := collectSrcs(files, cfg, matches)
	if !reflect.DeepEqual(libs, []string{"main.ts"}) {
		t.Errorf("libs = %v, want [main.ts]", libs)
	}
	if !reflect.DeepEqual(tests, []string{"main.test.ts"}) {
		t.Errorf("tests = %v, want [main.test.ts] (config-matched file must not appear as a test)", tests)
	}
}

// Default extensions are .ts/.tsx — a `vite.config.js` file mustn't be
// claimed by a `vite.config.*` config glob unless the user opts JS in via
// ts_extension. Otherwise the directive could silently swallow files the
// plugin otherwise ignores.
func TestMatchConfigFiles_NonTypeScriptIgnored(t *testing.T) {
	cfg := newTsConfig()
	cfg.configFiles = []configFileSpec{{pattern: "vite.config.*", attr: "vite_config_deps"}}
	got := matchConfigFiles([]string{"vite.config.js", "vite.config.ts"}, cfg)
	want := map[string]string{"vite.config.ts": "vite_config_deps"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("matchConfigFiles = %v, want %v (.js excluded with default extensions)", got, want)
	}

	// With .js opted in, the same call now claims it.
	cfg.extensions = append(cfg.extensions, ".js")
	got = matchConfigFiles([]string{"vite.config.js", "vite.config.ts"}, cfg)
	want = map[string]string{"vite.config.js": "vite_config_deps", "vite.config.ts": "vite_config_deps"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("matchConfigFiles = %v, want %v (.js included after ts_extension .js)", got, want)
	}
}
