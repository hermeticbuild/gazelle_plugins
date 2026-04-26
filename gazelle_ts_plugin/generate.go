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

	libSrcs, testSrcs := collectSrcs(args.RegularFiles, cfg)

	var tsFiles []string
	for _, f := range args.RegularFiles {
		if isTypeScriptFile(f, cfg) {
			tsFiles = append(tsFiles, filepath.Join(args.Dir, f))
		}
	}

	var sourceImports, testImports []ImportStatement
	if len(tsFiles) > 0 {
		allImports, _ := l.extractImportsBatch(tsFiles)
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

	if len(libSrcs) == 0 && len(testSrcs) == 0 {
		return language.GenerateResult{}
	}

	var genRules []*rule.Rule
	var genImports []interface{}

	if len(libSrcs) > 0 {
		r := rule.NewRule(cfg.libraryKind, cfg.libraryName)
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
		r := rule.NewRule(cfg.testKind, cfg.testName)
		data := append([]string{}, testSrcs...)
		data = append(data, cfg.testData...)
		// When both a library and a test rule exist in the same directory,
		// pull the library into the test's data so relative imports
		// (./index.js, ./util.js, …) resolve to its compiled output.
		if len(libSrcs) > 0 {
			data = append(data, ":"+cfg.libraryName)
		}
		r.SetAttr("data", data)
		if cfg.testEntryPoint != "" {
			r.SetAttr("entry_point", cfg.testEntryPoint)
		} else {
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

	return language.GenerateResult{
		Gen:     genRules,
		Imports: genImports,
	}
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
