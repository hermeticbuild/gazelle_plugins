// Package ts implements a Gazelle language extension for TypeScript packages.
//
// It generates stock rules_ts/rules_js rules:
//
//   - ts_project (from @aspect_rules_ts//ts:defs.bzl) for libraries
//   - js_test    (from @aspect_rules_js//js:defs.bzl) for tests
//
// Callsites that want their own macros wrap the generated kinds via
// # gazelle:map_kind, e.g.
//
//	# gazelle:map_kind ts_project myrepo_ts_library //tools:ts.bzl
//	# gazelle:map_kind js_test    myrepo_ts_test    //tools:ts.bzl
//
// The plugin operates in Gazelle's three-phase pipeline:
//
//  1. GenerateRules (generate.go): scan .ts/.tsx files, extract imports via
//     the import_extractor cgo FFI, create/update rules.
//  2. Imports (imports.go): register rules in the RuleIndex so other
//     packages can resolve their imports against ours.
//  3. Resolve (resolve.go): convert parsed imports into Bazel deps labels.
//
// All configuration lives in BUILD-file directives (see configure.go); see
// README.md for the full list and examples.
package ts

import (
	"github.com/bazelbuild/bazel-gazelle/label"
	"github.com/bazelbuild/bazel-gazelle/language"
	"github.com/bazelbuild/bazel-gazelle/rule"
)

// languageName is the unique identifier for this Gazelle extension. It must
// match the prefix used in directive keys (ts_enabled, ts_library_name, …).
const languageName = "ts"

// tsLang implements the language.Language interface from Gazelle. It carries
// cached package.json data used during import resolution.
type tsLang struct {
	// packageDeps is a set of all npm package names from the root package.json
	// (dependencies + devDependencies + optionalDependencies).
	packageDeps map[string]bool

	// subpathImportsMap stores the "imports" field from the root package.json,
	// e.g. `"#packages/*": "./packages/*"`. Augmented per-tree by
	// # gazelle:ts_subpath_import directives in configure.go.
	subpathImportsMap map[string]string
}

// NewLanguage creates a new TypeScript Gazelle language extension.
func NewLanguage() language.Language {
	return &tsLang{
		packageDeps:       make(map[string]bool),
		subpathImportsMap: make(map[string]string),
	}
}

func (l *tsLang) Name() string { return languageName }

// Embeds returns nil — TypeScript doesn't use Bazel's rule embedding mechanism.
func (l *tsLang) Embeds(r *rule.Rule, from label.Label) []label.Label { return nil }
