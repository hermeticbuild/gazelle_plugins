package ts

// Default values applied when no directive overrides them. These match the
// shape a "small typical" TS package emits with stock rules_ts/rules_js.
const (
	// Empty default means "use the package's directory basename" — see
	// resolveRuleNames in generate.go. This way //apps/web:web shortens to
	// //apps/web, the most natural Bazel idiom.
	defaultLibraryName       = ""
	defaultTestName          = ""
	defaultLibraryKind       = "ts_project"
	defaultTestKind          = "js_test"
	defaultNpmLinkPattern    = "//:node_modules/{pkg}"
	defaultProjectReferences = true
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

	// libraryKind / testKind are the rule kinds emitted. Stock defaults are
	// `ts_project` and `js_test`; override via directive when you'd rather
	// emit a different kind directly than rewrite via `# gazelle:map_kind`.
	libraryKind string
	testKind    string

	// visibility is the list of labels emitted on the library rule.
	visibility []string

	// testPatterns: glob-style patterns deciding which files are tests.
	testPatterns []string

	// extensions: file extensions treated as TypeScript source.
	extensions []string

	// projectReferences toggles emission of `composite = True` and the
	// `references` resolve attr on libraries.
	projectReferences bool

	// tsconfig: when set, every emitted ts_project gets this label as its
	// `tsconfig` attr. Empty = unset.
	tsconfig string

	// transpiler: when set, emitted as the `transpiler` attr on every
	// ts_project. rules_ts requires a transpiler selection; common values
	// are `partial(@aspect_rules_ts//ts:defs.bzl%tsc)` (a .bzl call) or a
	// custom macro name like `swc`.
	transpiler string

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

	// testEntryPoint, when set, is emitted as the test rule's `entry_point`.
	testEntryPoint string
}

// newTsConfig returns a config populated with all defaults.
func newTsConfig() *tsConfig {
	return &tsConfig{
		enabled:           true,
		libraryName:       defaultLibraryName,
		testName:          defaultTestName,
		libraryKind:       defaultLibraryKind,
		testKind:          defaultTestKind,
		visibility:        append([]string(nil), defaultVisibility...),
		testPatterns:      append([]string(nil), defaultTestPatterns...),
		extensions:        append([]string(nil), defaultExtensions...),
		projectReferences: defaultProjectReferences,
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
	return &cp
}
