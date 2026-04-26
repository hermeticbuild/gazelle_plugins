package ts

import (
	"flag"
	"strings"

	"github.com/bazelbuild/bazel-gazelle/config"
	"github.com/bazelbuild/bazel-gazelle/rule"
)

// All directives this plugin recognizes. Keep in sync with README.md.
const (
	directiveEnabled         = "ts_enabled"
	directiveLibraryName     = "ts_library_name"
	directiveTestName        = "ts_test_name"
	directiveLibraryKind     = "ts_library_kind"
	directiveTestKind        = "ts_test_kind"
	directiveVisibility      = "ts_visibility"
	directiveTestPattern     = "ts_test_pattern"
	directiveExtension       = "ts_extension"
	directiveProjectRefs     = "ts_project_references"
	directiveTsconfig        = "ts_tsconfig"
	directiveTranspiler      = "ts_transpiler"
	directiveNpmLinkPattern  = "ts_npm_link_pattern"
	directiveGeneratedPackage = "ts_generated_package"
	directiveTestData        = "ts_test_data"
	directiveTestEntryPoint  = "ts_test_entry_point"
)

// RegisterFlags is a no-op — all configuration is via BUILD-file directives.
func (l *tsLang) RegisterFlags(fs *flag.FlagSet, cmd string, c *config.Config) {}

// CheckFlags is a no-op — there are no flags to validate.
func (l *tsLang) CheckFlags(fs *flag.FlagSet, c *config.Config) error { return nil }

// KnownDirectives returns the directive keys this plugin reads. Gazelle
// silently ignores any directive whose key isn't in this list.
func (l *tsLang) KnownDirectives() []string {
	return []string{
		directiveEnabled,
		directiveLibraryName,
		directiveTestName,
		directiveLibraryKind,
		directiveTestKind,
		directiveVisibility,
		directiveTestPattern,
		directiveExtension,
		directiveProjectRefs,
		directiveTsconfig,
		directiveTranspiler,
		directiveNpmLinkPattern,
		directiveGeneratedPackage,
		directiveTestData,
		directiveTestEntryPoint,
	}
}

// Configure builds the per-directory config by cloning the parent config and
// applying any directives present in the current BUILD file. At the repo root
// it also loads package.json data for import resolution.
func (l *tsLang) Configure(c *config.Config, rel string, f *rule.File) {
	var cfg *tsConfig
	if raw, ok := c.Exts[languageName]; ok {
		cfg = raw.(*tsConfig).clone()
	} else {
		cfg = newTsConfig()
	}

	if f != nil {
		for _, d := range f.Directives {
			applyDirective(cfg, d)
		}
	}

	c.Exts[languageName] = cfg

	if rel == "" {
		l.loadPackageJSONDeps(c.RepoRoot)
		// Merge directive-supplied generated-package mappings on top of the
		// package.json imports map. Directives win on key collisions.
		for k, v := range cfg.generatedPackages {
			l.subpathImportsMap[k] = v
		}
	}
}

func applyDirective(cfg *tsConfig, d rule.Directive) {
	val := strings.TrimSpace(d.Value)
	switch d.Key {
	case directiveEnabled:
		cfg.enabled = parseBool(val, cfg.enabled)
	case directiveLibraryName:
		if val != "" {
			cfg.libraryName = val
		}
	case directiveTestName:
		if val != "" {
			cfg.testName = val
		}
	case directiveLibraryKind:
		if val != "" {
			cfg.libraryKind = val
		}
	case directiveTestKind:
		if val != "" {
			cfg.testKind = val
		}
	case directiveVisibility:
		if val != "" {
			cfg.visibility = splitFields(val)
		}
	case directiveTestPattern:
		if val != "" {
			cfg.testPatterns = appendUnique(cfg.testPatterns, val)
		}
	case directiveExtension:
		if val != "" {
			cfg.extensions = appendUnique(cfg.extensions, val)
		}
	case directiveProjectRefs:
		cfg.projectReferences = parseBool(val, cfg.projectReferences)
	case directiveTsconfig:
		cfg.tsconfig = val
	case directiveTranspiler:
		cfg.transpiler = val
	case directiveNpmLinkPattern:
		if val != "" {
			cfg.npmLinkPattern = val
		}
	case directiveGeneratedPackage:
		// Format: `<pattern>=<target>`, e.g. `@myrepo_generated/*=//:node_modules/@myrepo_generated/*`.
		// Maps a synthetic / generated package namespace to a Bazel label so
		// the resolver can emit a dep without the package being in package.json.
		if eq := strings.Index(val, "="); eq > 0 {
			pattern := strings.TrimSpace(val[:eq])
			target := strings.TrimSpace(val[eq+1:])
			if pattern != "" && target != "" {
				cfg.generatedPackages[pattern] = target
			}
		}
	case directiveTestData:
		if val != "" {
			cfg.testData = appendUnique(cfg.testData, val)
		}
	case directiveTestEntryPoint:
		cfg.testEntryPoint = val
	}
}

func parseBool(val string, fallback bool) bool {
	switch strings.ToLower(val) {
	case "true", "1", "yes", "on":
		return true
	case "false", "0", "no", "off":
		return false
	}
	return fallback
}

func splitFields(s string) []string {
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return nil
	}
	return fields
}

func appendUnique(slice []string, val string) []string {
	for _, v := range slice {
		if v == val {
			return slice
		}
	}
	return append(slice, val)
}
