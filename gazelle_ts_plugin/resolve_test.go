package ts

import (
	"reflect"
	"testing"
)

func TestMatchNpmPackage_Bare(t *testing.T) {
	deps := map[string]bool{"react": true, "lodash": true}
	cases := map[string]string{
		"react":            "react",
		"react/jsx-runtime": "react",
		"unknown":          "",
		"lodash/debounce":  "lodash",
	}
	for imp, want := range cases {
		if got := matchNpmPackage(imp, deps); got != want {
			t.Errorf("matchNpmPackage(%q) = %q, want %q", imp, got, want)
		}
	}
}

func TestMatchNpmPackage_Scoped(t *testing.T) {
	deps := map[string]bool{
		"@tanstack/react-query": true,
		"@mui/material":         true,
	}
	cases := map[string]string{
		"@tanstack/react-query":          "@tanstack/react-query",
		"@tanstack/react-query/devtools": "@tanstack/react-query",
		"@mui/material":                  "@mui/material",
		"@unknown/pkg":                   "",
		"@scopeonly":                     "", // missing slash
	}
	for imp, want := range cases {
		if got := matchNpmPackage(imp, deps); got != want {
			t.Errorf("matchNpmPackage(%q) = %q, want %q", imp, got, want)
		}
	}
}

func TestMatchNpmPackage_TypesFallback(t *testing.T) {
	// Type-only imports may resolve via @types/<pkg> when the runtime pkg
	// isn't a direct dep.
	deps := map[string]bool{"@types/lodash": true}
	got := matchNpmPackage("lodash", deps)
	if got != "@types/lodash" {
		t.Errorf("expected @types fallback, got %q", got)
	}
}

func TestTypesPackageFor(t *testing.T) {
	deps := map[string]bool{
		"@types/react":             true,
		"@types/lodash":            true,
		"@types/tanstack__react-query": true,
	}
	cases := map[string]string{
		"react":                 "@types/react",
		"lodash":                "@types/lodash",
		"@tanstack/react-query": "@types/tanstack__react-query",
		"unknown":               "",
	}
	for pkg, want := range cases {
		if got := typesPackageFor(pkg, deps); got != want {
			t.Errorf("typesPackageFor(%q) = %q, want %q", pkg, got, want)
		}
	}
}

func TestNpmLabel(t *testing.T) {
	cases := []struct {
		pattern string
		pkg     string
		want    string
	}{
		{"//:node_modules/{pkg}", "react", "//:node_modules/react"},
		{"//:node_modules/{pkg}", "@mui/material", "//:node_modules/@mui/material"},
		{"//pnpm:node_modules/{pkg}", "react", "//pnpm:node_modules/react"},
	}
	for _, c := range cases {
		cfg := &tsConfig{npmLinkPattern: c.pattern}
		got := npmLabel(cfg, c.pkg)
		if got != c.want {
			t.Errorf("npmLabel(%q, %q) = %q, want %q", c.pattern, c.pkg, got, c.want)
		}
	}
}

func TestDeduplicateAndSort(t *testing.T) {
	cases := []struct {
		in   []string
		want []string
	}{
		{nil, nil},
		{[]string{}, nil},
		{[]string{"b", "a", "b", "c"}, []string{"a", "b", "c"}},
		{[]string{"x"}, []string{"x"}},
	}
	for _, c := range cases {
		got := deduplicateAndSort(c.in)
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("deduplicateAndSort(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestNodeBuiltinsCovered(t *testing.T) {
	// Spot-check a few common ones to ensure the table didn't drift.
	for _, mod := range []string{"fs", "path", "crypto", "child_process", "events"} {
		if !nodeBuiltinModules[mod] {
			t.Errorf("expected %q in nodeBuiltinModules", mod)
		}
	}
}
