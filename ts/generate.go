package ts

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/bazelbuild/bazel-gazelle/language"
	"github.com/bazelbuild/bazel-gazelle/rule"
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
	libSrcs, testSrcs := collectSrcs(args.RegularFiles, cfg)

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
	allImports := map[string][]ImportStatement{}
	if len(tsFiles) > 0 {
		allImports, _ = l.extractImportsBatch(tsFiles)
		for _, f := range args.RegularFiles {
			if !isTypeScriptFile(f, cfg) {
				continue
			}
			fullPath := filepath.Join(args.Dir, f)
			imps := allImports[fullPath]
			if isTestFile(f, cfg) {
				testImports = append(testImports, imps...)
			} else {
				sourceImports = append(sourceImports, imps...)
			}
		}
	}

	if len(libSrcs) == 0 && len(testSrcs) == 0 && len(jsBinaries) == 0 {
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

// collectSrcs partitions the directory's files into library and test srcs,
// each sorted for deterministic BUILD output.
func collectSrcs(regularFiles []string, cfg *tsConfig) (libFiles, testFiles []string) {
	for _, f := range regularFiles {
		if isTestFile(f, cfg) {
			if isTypeScriptFile(f, cfg) {
				testFiles = append(testFiles, f)
			}
		} else if isTypeScriptFile(f, cfg) {
			libFiles = append(libFiles, f)
		}
	}
	sort.Strings(libFiles)
	sort.Strings(testFiles)
	return
}
