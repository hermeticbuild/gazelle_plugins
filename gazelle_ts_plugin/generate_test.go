package ts

import (
	"reflect"
	"testing"
)

func TestIsTypeScriptFile(t *testing.T) {
	cfg := newTsConfig()
	cases := map[string]bool{
		"a.ts":    true,
		"a.tsx":   true,
		"a.js":    false,
		"a.json":  false,
		"a.d.ts":  true,
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
		"foo.test.ts":      true,
		"foo.test.tsx":     true,
		"tests/index.ts":   true,
		"test/main.ts":     true,
		"src/foo.ts":       false,
		"foo.spec.ts":      false, // not in default patterns
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
		name           string
		cfg            *tsConfig
		rel            string
		wantLib        string
		wantTest       string
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
	libs, tests := collectSrcs(files, cfg)

	wantLibs := []string{"helper.ts", "main.ts", "types.tsx"}
	wantTests := []string{"main.test.ts", "tests/integration.ts"}
	if !reflect.DeepEqual(libs, wantLibs) {
		t.Errorf("libs = %v, want %v", libs, wantLibs)
	}
	if !reflect.DeepEqual(tests, wantTests) {
		t.Errorf("tests = %v, want %v", tests, wantTests)
	}
}
