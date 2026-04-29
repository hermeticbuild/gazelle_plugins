package ts

// Default values applied when no directive overrides them. These match the
// shape a "small typical" TS package emits with stock rules_ts/rules_js.
const (
	// Empty default means "use the package's directory basename" — see
	// resolveRuleNames in generate.go. This way //apps/web:web shortens to
	// //apps/web, the most natural Bazel idiom.
	defaultLibraryName    = ""
	defaultTestName       = ""
	defaultNpmLinkPattern = "//:node_modules/{pkg}"

	// KindTsLibrary is the abstract library kind the plugin emits. Consumers
	// must `# gazelle:map_kind ts_library <macro> <load_path>` to a concrete
	// macro; the fallback in @gazelle_ts//ts:defs.bzl is a filegroup that
	// keeps the BUILD parseable but doesn't typecheck.
	KindTsLibrary = "ts_library"

	// KindTsTest is the abstract test kind. The plugin emits it with no
	// entry_point — consumers map_kind to vitest_test, jest_test, or any
	// multi-entry runner that auto-discovers from `data`. The fallback in
	// @gazelle_ts//ts:defs.bzl is a filegroup so the BUILD still loads.
	KindTsTest = "ts_test"
)

// Default test-file patterns and source-file extensions. Patterns are matched
// against the file path relative to the package directory.
var (
	defaultTestPatterns = []string{"*.test.ts", "*.test.tsx", "tests/**", "test/**"}
	defaultExtensions   = []string{".ts", ".tsx"}
	defaultVisibility   = []string{"//visibility:public"}
)

// tsConfig holds per-directory configuration. Gazelle calls Configure() for
// each directory during the walk, building up the config by cloning the
// parent and applying any directives in the directory's BUILD file.
type tsConfig struct {
	enabled bool

	// libraryName / testName are the names of the generated rules.
	libraryName string
	testName    string

	// visibility is the list of labels emitted on the library rule.
	visibility []string

	// testPatterns: glob-style patterns deciding which files are tests.
	testPatterns []string

	// extensions: file extensions treated as TypeScript source.
	extensions []string

	// npmLinkPattern is the template used for npm package labels, e.g.
	// `//:node_modules/{pkg}`. The literal `{pkg}` is replaced with the
	// resolved package name.
	npmLinkPattern string

	// generatedPackages maps synthetic/generated package namespaces to Bazel
	// labels. Pattern → target replacement (e.g.
	// `@myrepo_generated/*` → `//:node_modules/@myrepo_generated/*`). Merged
	// on top of the package.json `imports` map at the repo root.
	generatedPackages map[string]string

	// testData is added to every emitted test rule's `data` attr.
	testData []string

	// bundlerConfigSpecs lists the bundler/tooling config files held out of
	// the library compilation unit, each with its own emitted target name.
	// Each spec maps a glob pattern to the Bazel target name to emit; matched
	// files are excluded from libSrcs/testSrcs and grouped under their spec.
	bundlerConfigSpecs []bundlerConfigSpec
}

// bundlerConfigSpec is one entry of the ts_bundler_config_pattern directive:
// a glob plus the target name to emit for files matching that glob.
type bundlerConfigSpec struct {
	Pattern string
	Name    string
}

// newTsConfig returns a config populated with all defaults.
func newTsConfig() *tsConfig {
	return &tsConfig{
		enabled:           true,
		libraryName:       defaultLibraryName,
		testName:          defaultTestName,
		visibility:        append([]string(nil), defaultVisibility...),
		testPatterns:      append([]string(nil), defaultTestPatterns...),
		extensions:        append([]string(nil), defaultExtensions...),
		npmLinkPattern:    defaultNpmLinkPattern,
		generatedPackages: make(map[string]string),
	}
}

// clone makes a deep copy so child directories inherit but can override
// without mutating the parent.
func (c *tsConfig) clone() *tsConfig {
	cp := *c
	cp.visibility = append([]string(nil), c.visibility...)
	cp.testPatterns = append([]string(nil), c.testPatterns...)
	cp.extensions = append([]string(nil), c.extensions...)
	cp.testData = append([]string(nil), c.testData...)
	cp.generatedPackages = make(map[string]string, len(c.generatedPackages))
	for k, v := range c.generatedPackages {
		cp.generatedPackages[k] = v
	}
	cp.bundlerConfigSpecs = append([]bundlerConfigSpec(nil), c.bundlerConfigSpecs...)
	return &cp
}
