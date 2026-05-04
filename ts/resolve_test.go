package ts

import (
	"encoding/json"
	"flag"
	"os"
	"reflect"
	"testing"

	"github.com/bazelbuild/bazel-gazelle/config"
	"github.com/bazelbuild/bazel-gazelle/label"
	gazelleresolve "github.com/bazelbuild/bazel-gazelle/resolve"
	"github.com/bazelbuild/bazel-gazelle/rule"
)

func TestMatchNpmPackage_Bare(t *testing.T) {
	deps := map[string]bool{"react": true, "lodash": true}
	cases := map[string]string{
		"react":             "react",
		"react/jsx-runtime": "react",
		"unknown":           "",
		"lodash/debounce":   "lodash",
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
		"@types/react":                 true,
		"@types/lodash":                true,
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

func TestMatchSubpathImportPattern_NonSuffixWildcard(t *testing.T) {
	capture, ok := matchSubpathImportPattern("#generated/typespec/rest/*/index.js", "#generated/typespec/rest/users/index.js")
	if !ok {
		t.Fatalf("matchSubpathImportPattern did not match")
	}
	if capture != "users" {
		t.Errorf("capture = %q, want users", capture)
	}

	if _, ok := matchSubpathImportPattern("#generated/typespec/rest/*/index.js", "#generated/typespec/rest/users/client.js"); ok {
		t.Errorf("unexpected match for non-index import")
	}
}

func TestMatchSubpathImportPattern_NodeStarCanContainSlash(t *testing.T) {
	capture, ok := matchSubpathImportPattern("#generated/protobuf/*.js", "#generated/protobuf/foo/bar/baz.js")
	if !ok {
		t.Fatalf("matchSubpathImportPattern did not match")
	}
	if capture != "foo/bar/baz" {
		t.Errorf("capture = %q, want foo/bar/baz", capture)
	}
}

func TestResolveSubpathImport_LabelTemplateFromPackageImports(t *testing.T) {
	lang := &tsLang{
		packageDeps: map[string]bool{},
		subpathImportsMap: map[string][]string{
			"#generated/typespec/rest/*/index.js": {"//typespec/rest/*:*.web"},
		},
	}

	got, external := lang.resolveSubpathImport(
		"#generated/typespec/rest/users/index.js",
		label.Label{Pkg: "apps/web", Name: "web"},
		nil,
	)
	if !external {
		t.Fatalf("external = false, want true")
	}
	if got != "//typespec/rest/users:users.web" {
		t.Errorf("resolveSubpathImport = %q, want //typespec/rest/users:users.web", got)
	}
}

func TestResolveSubpathImport_PathTargetUsesRuleIndex(t *testing.T) {
	lang := &tsLang{
		packageDeps: map[string]bool{},
		subpathImportsMap: map[string][]string{
			"#generated/typespec/rest/*/index.js": {"./typespec/rest/*"},
		},
	}
	c := config.New()
	c.Exts[languageName] = newTsConfig()
	ix := gazelleresolve.NewRuleIndex(func(r *rule.Rule, pkgRel string) gazelleresolve.Resolver {
		if r.Kind() == KindTsLibrary {
			return lang
		}
		return nil
	})
	ix.AddRule(c, rule.NewRule(KindTsLibrary, "users.web"), &rule.File{Pkg: "typespec/rest/users"})
	ix.Finish()

	got, external := lang.resolveSubpathImport(
		"#generated/typespec/rest/users/index.js",
		label.Label{Pkg: "app", Name: "app"},
		ix,
	)
	if external {
		t.Fatalf("external = true, want false")
	}
	if got != "//typespec/rest/users:users.web" {
		t.Errorf("resolveSubpathImport = %q, want //typespec/rest/users:users.web", got)
	}
}

func TestResolveImportsToDeps_ExactOverridePrecedesPackageImports(t *testing.T) {
	cfg := newTsConfig()
	c := config.New()
	c.Exts[languageName] = cfg
	resolveConfigurer := &gazelleresolve.Configurer{}
	resolveConfigurer.RegisterFlags(flag.NewFlagSet("test", flag.ContinueOnError), "", c)
	resolveConfigurer.Configure(c, "", &rule.File{
		Directives: []rule.Directive{
			ruleDirective("resolve", "ts #generated/types/user.js //exact:dep"),
		},
	})

	lang := &tsLang{
		packageDeps: map[string]bool{},
		subpathImportsMap: map[string][]string{
			"#generated/*": {"//generated:*"},
		},
	}
	got := lang.resolveImportsToDeps(
		c,
		[]ImportStatement{{ImportPath: "#generated/types/user.js"}},
		label.Label{Pkg: "apps/web", Name: "web"},
		nil,
		cfg,
	)
	want := []string{"//exact:dep"}
	if !reflect.DeepEqual(got.external, want) {
		t.Errorf("external deps = %v, want %v", got.external, want)
	}
}

func TestLoadPackageJSONDeps_ArrayFallbackImports(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(dir+"/package.json", []byte(`{
  "dependencies": {
    "react": "latest"
  },
  "imports": {
    "#generated/foo/*": [
      "./bazel-bin/generated/foo/dist/*",
      "./generated/foo/dist/*"
    ],
    "#conditional/*": {
      "browser": "./browser/*",
      "node": {
        "require": "./node-cjs/*",
        "import": "./node-esm/*"
      },
      "default": "./default/*"
    },
    "#null/*": null,
    "#unsupported/*": [42, null]
  }
}`), 0o644); err != nil {
		t.Fatal(err)
	}

	lang := &tsLang{
		packageDeps:       map[string]bool{},
		subpathImportsMap: map[string][]string{},
	}
	lang.loadPackageJSONDeps(dir)

	if !lang.packageDeps["react"] {
		t.Fatalf("dependencies were not loaded")
	}
	want := []string{
		"./bazel-bin/generated/foo/dist/*",
		"./generated/foo/dist/*",
	}
	if got := lang.subpathImportsMap["#generated/foo/*"]; !reflect.DeepEqual(got, want) {
		t.Errorf("imports fallback targets = %v, want %v", got, want)
	}
	if got := lang.subpathImportsMap["#conditional/*"]; !reflect.DeepEqual(got, []string{"./node-esm/*"}) {
		t.Errorf("conditional imports targets = %v, want [./node-esm/*]", got)
	}
	if _, ok := lang.subpathImportsMap["#null/*"]; ok {
		t.Errorf("null imports entry should be ignored")
	}
	if _, ok := lang.subpathImportsMap["#unsupported/*"]; ok {
		t.Errorf("unsupported imports entry should be ignored")
	}
}

func TestDecodePackageImportTargets_ConditionOrder(t *testing.T) {
	raw := json.RawMessage(`{
  "default": "./default.js",
  "node": "./node.js"
}`)
	got := decodePackageImportTargets(raw)
	want := []string{"./default.js"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("decodePackageImportTargets = %v, want %v", got, want)
	}
}

func TestDecodePackageImportTargets_ArrayFallbacks(t *testing.T) {
	raw := json.RawMessage(`[
  null,
  42,
  {"browser": "./browser.js", "default": "./default.js"},
  "./last.js"
]`)
	got := decodePackageImportTargets(raw)
	want := []string{"./default.js", "./last.js"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("decodePackageImportTargets = %v, want %v", got, want)
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

func ruleDirective(key, value string) rule.Directive {
	return rule.Directive{Key: key, Value: value}
}

func TestNodeBuiltinsCovered(t *testing.T) {
	// Spot-check a few common ones to ensure the table didn't drift.
	for _, mod := range []string{"fs", "path", "crypto", "child_process", "events"} {
		if !nodeBuiltinModules[mod] {
			t.Errorf("expected %q in nodeBuiltinModules", mod)
		}
	}
}
