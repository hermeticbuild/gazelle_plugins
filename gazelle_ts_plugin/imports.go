package ts

import (
	"github.com/bazelbuild/bazel-gazelle/config"
	"github.com/bazelbuild/bazel-gazelle/resolve"
	"github.com/bazelbuild/bazel-gazelle/rule"
)

// Imports returns the import specs that a rule provides; gazelle stores these
// in a reverse index that maps import paths to Bazel labels.
//
// For a library at //packages/foo, we register:
//   - "packages/foo"   (exact match)
//   - "packages/foo/*" (wildcard for subpath imports)
//
// This lets Resolve() answer queries like
// `#packages/foo/bar.js` → //packages/foo and `packages/foo` → //packages/foo.
//
// Test rules don't export reusable modules, so they don't appear in the index.
func (l *tsLang) Imports(c *config.Config, r *rule.Rule, f *rule.File) []resolve.ImportSpec {
	cfg, _ := c.Exts[languageName].(*tsConfig)
	if cfg == nil {
		cfg = newTsConfig()
	}

	// Match either the configured library kind or its post-map_kind name.
	if r.Kind() != cfg.libraryKind {
		return nil
	}

	pkg := f.Pkg
	return []resolve.ImportSpec{
		{Lang: languageName, Imp: pkg},
		{Lang: languageName, Imp: pkg + "/*"},
	}
}
