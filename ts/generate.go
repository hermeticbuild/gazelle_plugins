package ts

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/bazelbuild/bazel-gazelle/language"
	"github.com/bazelbuild/bazel-gazelle/rule"
	"github.com/bmatcuk/doublestar/v4"
)

// ImportData carries parsed imports from GenerateRules to Resolve. Gazelle
// runs GenerateRules during the directory walk (before the RuleIndex is
// complete) and Resolve afterwards, so we stash everything we'll need here.
type ImportData struct {
	Imports     []ImportStatement // source-file imports
	TestImports []ImportStatement // test-file imports
}

// GenerateRules walks a directory's files, partitions them into source vs.
// test, parses imports via the Rust subprocess, and emits library + test
// rules. The merge engine reconciles the result with the existing BUILD
// content using KindInfo from kinds.go.
func (l *tsLang) GenerateRules(args language.GenerateArgs) language.GenerateResult {
	cfg, ok := args.Config.Exts[languageName].(*tsConfig)
	if !ok || !cfg.enabled {
		return language.GenerateResult{}
	}

	libName, testName := resolveRuleNames(cfg, args.Rel)
	parts := collectSrcs(args.RegularFiles, cfg)
	libSrcs := parts.lib
	testSrcs := parts.test

	var tsFiles []string
	for _, f := range args.RegularFiles {
		if isTypeScriptFile(f, cfg) {
			tsFiles = append(tsFiles, filepath.Join(args.Dir, f))
		}
	}

	// Hand-written js_binary rules in this BUILD that we manage `data` for.
	// We never generate js_binary; we only piggyback on the user's existing
	// rule, scan its entry_point/srcs for imports, and let Resolve set deps.
	type jsBinaryRef struct {
		name  string
		files []string // package-relative TS files referenced by the rule
	}
	var jsBinaries []jsBinaryRef
	if args.File != nil {
		seen := make(map[string]bool, len(tsFiles))
		for _, f := range tsFiles {
			seen[f] = true
		}
		for _, r := range args.File.Rules {
			if r.Kind() != KindJsBinary {
				continue
			}
			ref := jsBinaryRef{name: r.Name()}
			candidates := append([]string{r.AttrString("entry_point")}, r.AttrStrings("srcs")...)
			for _, c := range candidates {
				if c == "" || !isTypeScriptFile(c, cfg) {
					continue
				}
				ref.files = append(ref.files, c)
				full := filepath.Join(args.Dir, c)
				if !seen[full] {
					tsFiles = append(tsFiles, full)
					seen[full] = true
				}
			}
			if len(ref.files) > 0 {
				jsBinaries = append(jsBinaries, ref)
			}
		}
	}

	var sourceImports, testImports []ImportStatement
	bundlerImportsBySpec := map[int][]ImportStatement{}
	allImports := map[string][]ImportStatement{}
	if len(tsFiles) > 0 {
		allImports, _ = l.extractImportsBatch(tsFiles)
		for _, f := range args.RegularFiles {
			if !isTypeScriptFile(f, cfg) {
				continue
			}
			fullPath := filepath.Join(args.Dir, f)
			imps := allImports[fullPath]
			// Bundler-config classification wins over test classification —
			// matches collectSrcs.
			if idx, ok := matchBundlerConfigSpec(f, cfg); ok {
				bundlerImportsBySpec[idx] = append(bundlerImportsBySpec[idx], imps...)
				continue
			}
			if isTestFile(f, cfg) {
				testImports = append(testImports, imps...)
			} else {
				sourceImports = append(sourceImports, imps...)
			}
		}
	}

	if len(libSrcs) == 0 && len(testSrcs) == 0 && len(jsBinaries) == 0 && len(parts.bundlerConfigs) == 0 {
		return language.GenerateResult{}
	}

	var genRules []*rule.Rule
	var genImports []interface{}

	if len(libSrcs) > 0 {
		r := rule.NewRule(cfg.libraryKind, libName)
		r.SetAttr("srcs", libSrcs)
		if len(cfg.visibility) > 0 {
			r.SetAttr("visibility", cfg.visibility)
		}
		if cfg.tsconfig != "" {
			r.SetAttr("tsconfig", cfg.tsconfig)
		}
		if cfg.projectReferences {
			// `composite = True` is what TypeScript reads as a project
			// reference. The other flags match the tsconfig validator's
			// expectations when the shared tsconfig has them on.
			r.SetAttr("composite", true)
			r.SetAttr("declaration", true)
			r.SetAttr("declaration_map", true)
			r.SetAttr("source_map", true)
		}
		genRules = append(genRules, r)
		genImports = append(genImports, ImportData{Imports: sourceImports})
	}

	if len(testSrcs) > 0 {
		// Stock js_test takes `data` (no srcs/deps). The entry_point is the
		// .js file node runs; data carries every input the test sandbox
		// needs (test source files, fixtures, npm packages, sibling lib).
		r := rule.NewRule(cfg.testKind, testName)
		data := append([]string{}, testSrcs...)
		data = append(data, cfg.testData...)
		// When both a library and a test rule exist in the same directory,
		// pull the library into the test's data so relative imports
		// (./index.js, ./util.js, …) resolve to its compiled output.
		if len(libSrcs) > 0 {
			data = append(data, ":"+libName)
		}
		r.SetAttr("data", data)
		// entry_point is required for stock js_test (Node needs a single
		// script to run). Test runners that auto-discover (vitest, jest,
		// mocha) don't need it — set ts_test_entry_point_auto=false to
		// suppress the auto-pick when you've mapped the kind to such a
		// runner.
		if cfg.testEntryPoint != "" {
			r.SetAttr("entry_point", cfg.testEntryPoint)
		} else if cfg.testEntryPointAuto {
			for _, s := range testSrcs {
				if strings.HasSuffix(s, ".test.ts") || strings.HasSuffix(s, ".test.tsx") {
					r.SetAttr("entry_point", s)
					break
				}
			}
		}
		genRules = append(genRules, r)
		genImports = append(genImports, ImportData{
			Imports:     sourceImports,
			TestImports: testImports,
		})
	}

	// Existing js_binary rules — emit a placeholder so Resolve runs against
	// each, but don't set any attrs. The merge engine keeps the user's
	// entry_point, srcs, env, etc.; Resolve only fills in `data`.
	for _, jb := range jsBinaries {
		var imps []ImportStatement
		for _, f := range jb.files {
			imps = append(imps, allImports[filepath.Join(args.Dir, f)]...)
		}
		genRules = append(genRules, rule.NewRule(KindJsBinary, jb.name))
		genImports = append(genImports, ImportData{Imports: imps})
	}

	// Bundler-config rules — one per spec target name. Multiple specs may
	// share a name (e.g. several patterns routed to a single `bundlers`
	// target); their files and imports are merged in directive order. Each
	// emitted rule resolves its own deps closure separately from the lib.
	type bundlerGroup struct {
		name    string
		srcs    []string
		imports []ImportStatement
	}
	var bundlerGroups []*bundlerGroup
	bundlerGroupsByName := map[string]*bundlerGroup{}
	for idx, spec := range cfg.bundlerConfigSpecs {
		files := parts.bundlerConfigs[idx]
		if len(files) == 0 {
			continue
		}
		g := bundlerGroupsByName[spec.Name]
		if g == nil {
			g = &bundlerGroup{name: spec.Name}
			bundlerGroupsByName[spec.Name] = g
			bundlerGroups = append(bundlerGroups, g)
		}
		g.srcs = append(g.srcs, files...)
		g.imports = append(g.imports, bundlerImportsBySpec[idx]...)
	}
	for _, g := range bundlerGroups {
		// Files are unique across specs (longest-pattern-wins), but if multiple
		// specs share a name, sort to keep srcs deterministic.
		sort.Strings(g.srcs)
		r := rule.NewRule(KindBundlerConfig, g.name)
		r.SetAttr("srcs", g.srcs)
		if len(cfg.visibility) > 0 {
			r.SetAttr("visibility", cfg.visibility)
		}
		if cfg.tsconfig != "" {
			r.SetAttr("tsconfig", cfg.tsconfig)
		}
		genRules = append(genRules, r)
		genImports = append(genImports, ImportData{Imports: g.imports})
	}

	return language.GenerateResult{
		Gen:     genRules,
		Imports: genImports,
	}
}

// resolveRuleNames returns the (library, test) rule names for a directory,
// applying the directive overrides if set or falling back to package-name-
// derived defaults.
//
// Defaults — given a package at //apps/web (rel = "apps/web"):
//
//	library: "web"      → //apps/web:web (Bazel shortens to //apps/web)
//	test:    "web_test" → //apps/web:web_test
//
// Both can be overridden per-tree via the ts_library_name / ts_test_name
// directives. At the repo root (rel = ""), where there's no basename to
// derive from, library falls back to "lib" and test to "test".
func resolveRuleNames(cfg *tsConfig, rel string) (libName, testName string) {
	base := filepath.Base(rel)
	if base == "." || base == "" || base == "/" {
		base = ""
	}

	libName = cfg.libraryName
	if libName == "" {
		if base != "" {
			libName = base
		} else {
			libName = "lib"
		}
	}

	testName = cfg.testName
	if testName == "" {
		if base != "" {
			testName = base + "_test"
		} else {
			testName = "test"
		}
	}
	return
}

// isTypeScriptFile checks the configured extensions list.
func isTypeScriptFile(name string, cfg *tsConfig) bool {
	for _, ext := range cfg.extensions {
		if strings.HasSuffix(name, ext) {
			return true
		}
	}
	return false
}

// isTestFile matches the file path against any of the configured test
// patterns. Patterns may contain `**` (matches across directories) and `*`
// (matches within a path segment).
func isTestFile(name string, cfg *tsConfig) bool {
	for _, pat := range cfg.testPatterns {
		if matchTestPattern(pat, name) {
			return true
		}
	}
	return false
}

// matchTestPattern is a small glob matcher supporting `*` (path segment) and
// `**` (path-spanning). We avoid filepath.Match because it doesn't support `**`.
func matchTestPattern(pattern, name string) bool {
	// Fast path for prefix-style patterns ("tests/**", "test/**").
	if strings.HasSuffix(pattern, "/**") {
		prefix := strings.TrimSuffix(pattern, "/**")
		return name == prefix || strings.HasPrefix(name, prefix+"/")
	}
	// `*.test.ts` style: substring match suffices because gazelle hands us
	// directory-local file names without paths.
	if strings.HasPrefix(pattern, "*") {
		return strings.HasSuffix(name, strings.TrimPrefix(pattern, "*"))
	}
	if strings.HasSuffix(pattern, "*") {
		return strings.HasPrefix(name, strings.TrimSuffix(pattern, "*"))
	}
	return name == pattern
}

// partitionedSrcs is the result of slicing a directory's TS files across the
// three roles a file can play: library source, test source, or bundler-config
// (one bucket per matched ts_bundler_config_pattern spec, keyed by spec
// index). Each slice is sorted for deterministic BUILD output.
type partitionedSrcs struct {
	lib            []string
	test           []string
	bundlerConfigs map[int][]string
}

// collectSrcs partitions the directory's files for emission. Bundler-config
// patterns take precedence over test patterns, so a file matching both goes
// to the bundler-config bucket — the boundary the directive enforces is
// stronger than the lib/test split.
func collectSrcs(regularFiles []string, cfg *tsConfig) partitionedSrcs {
	out := partitionedSrcs{bundlerConfigs: map[int][]string{}}
	for _, f := range regularFiles {
		if !isTypeScriptFile(f, cfg) {
			continue
		}
		if idx, ok := matchBundlerConfigSpec(f, cfg); ok {
			out.bundlerConfigs[idx] = append(out.bundlerConfigs[idx], f)
			continue
		}
		if isTestFile(f, cfg) {
			out.test = append(out.test, f)
			continue
		}
		out.lib = append(out.lib, f)
	}
	sort.Strings(out.lib)
	sort.Strings(out.test)
	for k, v := range out.bundlerConfigs {
		sort.Strings(v)
		out.bundlerConfigs[k] = v
	}
	return out
}

// matchBundlerConfigSpec returns the index of the longest-matching spec for
// the given file path (relative to the package), or -1, false. Longest
// pattern wins so a more-specific spec like `vite.config.production.ts`
// overrides a less-specific one like `vite.config.*` for the same file.
func matchBundlerConfigSpec(name string, cfg *tsConfig) (int, bool) {
	bestIdx := -1
	bestLen := -1
	for i, spec := range cfg.bundlerConfigSpecs {
		ok, err := doublestar.Match(spec.Pattern, name)
		if err != nil || !ok {
			continue
		}
		if len(spec.Pattern) > bestLen {
			bestLen = len(spec.Pattern)
			bestIdx = i
		}
	}
	return bestIdx, bestIdx >= 0
}
