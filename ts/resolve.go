package ts

import (
	"encoding/json"
	"os"
	"sort"
	"strings"

	"github.com/bazelbuild/bazel-gazelle/config"
	"github.com/bazelbuild/bazel-gazelle/label"
	"github.com/bazelbuild/bazel-gazelle/repo"
	"github.com/bazelbuild/bazel-gazelle/resolve"
	"github.com/bazelbuild/bazel-gazelle/rule"
)

// nodeBuiltinModules: imports of these (or `node:<name>`) resolve to
// `@types/node` rather than going through npm package lookup.
var nodeBuiltinModules = map[string]bool{
	"assert": true, "buffer": true, "child_process": true, "cluster": true,
	"crypto": true, "dgram": true, "dns": true, "events": true, "fs": true,
	"http": true, "http2": true, "https": true, "module": true, "net": true,
	"os": true, "path": true, "process": true, "querystring": true,
	"readline": true, "stream": true, "string_decoder": true, "timers": true,
	"tls": true, "tty": true, "url": true, "util": true, "v8": true,
	"vm": true, "worker_threads": true, "zlib": true, "console": true,
	"perf_hooks": true, "async_hooks": true, "diagnostics_channel": true,
	"inspector": true, "test": true, "trace_events": true, "wasi": true,
}

// packageJSON captures only the fields we read from the root package.json.
type packageJSON struct {
	Imports              map[string]string `json:"imports"`
	Dependencies         map[string]string `json:"dependencies"`
	DevDependencies      map[string]string `json:"devDependencies"`
	OptionalDependencies map[string]string `json:"optionalDependencies"`
}

// resolvedDeps holds the two categories we attach to a rule.
type resolvedDeps struct {
	internal []string // intra-repo labels → references / project references
	external []string // npm labels → deps
}

// Resolve converts ImportData (attached during GenerateRules) into Bazel
// labels and writes them onto the rule. Library rules get `deps` and
// `references`; test rules collapse both into `deps`.
func (l *tsLang) Resolve(
	c *config.Config,
	ix *resolve.RuleIndex,
	rc *repo.RemoteCache,
	r *rule.Rule,
	rawImportData interface{},
	from label.Label,
) {
	cfg, _ := c.Exts[languageName].(*tsConfig)
	if cfg == nil {
		cfg = newTsConfig()
	}
	importData, ok := rawImportData.(ImportData)
	if !ok {
		return
	}

	switch r.Kind() {
	case cfg.libraryKind:
		// Stock ts_project takes a single `deps` attr for both npm packages
		// and ts_project-to-ts_project project references — `composite` on
		// the dep is what TypeScript reads as a project reference.
		resolved := l.resolveImportsToDeps(c, importData.Imports, from, ix, cfg)
		all := append([]string{}, resolved.external...)
		all = append(all, resolved.internal...)
		setOrDelete(r, "deps", all)

	case cfg.testKind:
		// Stock js_test takes `data` for everything: every npm package, every
		// internal lib the test imports, plus the test sources themselves
		// (already added in GenerateRules). Merge into the existing data list.
		testResolved := l.resolveImportsToDeps(c, importData.TestImports, from, ix, cfg)
		srcResolved := l.resolveImportsToDeps(c, importData.Imports, from, ix, cfg)
		existing := r.AttrStrings("data")
		all := append([]string{}, existing...)
		all = append(all, testResolved.external...)
		all = append(all, testResolved.internal...)
		all = append(all, srcResolved.external...)
		all = append(all, srcResolved.internal...)
		setOrDelete(r, "data", all)

	case KindJsBinary:
		// We don't generate js_binary rules — only fill in their `data` attr
		// based on what their entry_point/srcs import. The user's existing
		// entry_point, env, fixed_args, etc. are left alone.
		resolved := l.resolveImportsToDeps(c, importData.Imports, from, ix, cfg)
		all := append([]string{}, resolved.external...)
		all = append(all, resolved.internal...)
		setOrDelete(r, "data", all)

	case KindBundlerConfig:
		// Bundler-config rules are a separate compilation unit so build-time
		// deps don't enter the lib's runtime closure. Resolution mirrors the
		// library, plus a sibling-lib link when the config imports any
		// relative file (e.g. `vite.config.ts` importing `./viteHelpers.ts`):
		// helpers stay in the lib target and the bundler-config depends on
		// it. The asymmetry is intentional — the closure leaks bundler→lib
		// but never lib→bundler.
		resolved := l.resolveImportsToDeps(c, importData.Imports, from, ix, cfg)
		all := append([]string{}, resolved.external...)
		all = append(all, resolved.internal...)
		for _, imp := range importData.Imports {
			if !strings.HasPrefix(imp.ImportPath, ".") {
				continue
			}
			spec := resolve.ImportSpec{Lang: languageName, Imp: from.Pkg}
			found := ix.FindRulesByImportWithConfig(c, spec, languageName)
			if len(found) == 0 {
				break
			}
			all = append(all, found[0].Label.Rel(from.Repo, from.Pkg).String())
			break
		}
		setOrDelete(r, "deps", all)
	}
}

func setOrDelete(r *rule.Rule, attr string, values []string) {
	values = deduplicateAndSort(values)
	if len(values) > 0 {
		r.SetAttr(attr, values)
	} else {
		r.DelAttr(attr)
	}
}

// resolveImportsToDeps categorizes each import into internal vs external.
func (l *tsLang) resolveImportsToDeps(
	c *config.Config,
	imports []ImportStatement,
	from label.Label,
	ix *resolve.RuleIndex,
	cfg *tsConfig,
) resolvedDeps {
	result := resolvedDeps{}
	seen := make(map[string]bool)

	for _, imp := range imports {
		if seen[imp.ImportPath] {
			continue
		}
		seen[imp.ImportPath] = true

		path := imp.ImportPath

		// Relative imports stay within the package; nothing to add.
		if strings.HasPrefix(path, ".") {
			continue
		}

		// First consult gazelle's resolve directive overrides so callsites
		// can route arbitrary imports with `# gazelle:resolve ts <import>
		// <label>`. Note: gazelle's RuleIndex.FindRulesByImportWithConfig
		// does NOT check overrides on its own (it only walks the rule
		// index and CrossResolvers), so we have to call FindRuleWithOverride
		// explicitly. Overrides win over every other resolution path.
		spec := resolve.ImportSpec{Lang: languageName, Imp: path}
		if dep, ok := resolve.FindRuleWithOverride(c, spec, languageName); ok {
			result.external = append(result.external, dep.Rel(from.Repo, from.Pkg).String())
			continue
		}

		// Subpath / generated-package imports — anything matching a key in
		// subpathImportsMap (sourced from package.json `imports` or
		// ts_generated_package). Literal Bazel labels (start with `//` or
		// `@`) go to `deps` because they're typically npm-style packages
		// (npm_package, js_library, …); workspace-path targets resolve via
		// the RuleIndex and go to `references` (TS project references).
		if target, external := l.resolveSubpathImport(path, from, ix); target != "" {
			if external {
				result.external = append(result.external, target)
			} else {
				result.internal = append(result.internal, target)
			}
			continue
		}

		// Node.js builtins (with or without `node:` prefix) → @types/node.
		modulePath := strings.TrimPrefix(path, "node:")
		baseModule := strings.Split(modulePath, "/")[0]
		if nodeBuiltinModules[baseModule] {
			result.external = append(result.external, npmLabel(cfg, "@types/node"))
			continue
		}

		// npm packages from package.json deps.
		if pkgName := matchNpmPackage(path, l.packageDeps); pkgName != "" {
			result.external = append(result.external, npmLabel(cfg, pkgName))
			if typesName := typesPackageFor(pkgName, l.packageDeps); typesName != "" {
				result.external = append(result.external, npmLabel(cfg, typesName))
			}
		}
	}

	result.internal = deduplicateAndSort(result.internal)
	result.external = deduplicateAndSort(result.external)
	return result
}

// resolveSubpathImport tries each key in subpathImportsMap (longest pattern
// first) and returns the matching Bazel label plus a flag indicating whether
// the label is external (deps) or internal (references). Empty label means
// no match.
func (l *tsLang) resolveSubpathImport(importPath string, from label.Label, ix *resolve.RuleIndex) (string, bool) {
	keys := make([]string, 0, len(l.subpathImportsMap))
	for k := range l.subpathImportsMap {
		keys = append(keys, k)
	}
	// Longest pattern wins so e.g. `#packages/foo/*` beats `#packages/*`.
	sort.Slice(keys, func(i, j int) bool { return len(keys[i]) > len(keys[j]) })

	for _, pattern := range keys {
		target := l.subpathImportsMap[pattern]
		prefix := strings.TrimSuffix(pattern, "*")
		if !strings.HasPrefix(importPath, prefix) {
			continue
		}

		// If the target itself is already a Bazel label, use it directly
		// (with the `*` substituted for the matched suffix). Literal labels
		// are treated as external deps — typical use is wiring a synthetic
		// npm-style package (npm_package, js_library) for things not in
		// package.json.
		if strings.HasPrefix(target, "//") || strings.HasPrefix(target, "@") {
			suffix := strings.TrimPrefix(importPath, prefix)
			out := strings.ReplaceAll(target, "*", suffix)
			if strings.Contains(out, "*") {
				out = strings.TrimSuffix(target, "*")
			}
			return out, true
		}

		// Otherwise treat target as a path within the repo and look up the
		// matching ts_project in the rule index.
		tp := strings.TrimSuffix(target, "*")
		tp = strings.TrimPrefix(tp, "./")
		resolvedPath := tp + strings.TrimPrefix(importPath, prefix)
		for _, ext := range []string{".js", ".ts", ".tsx", ".jsx"} {
			resolvedPath = strings.TrimSuffix(resolvedPath, ext)
		}
		parts := strings.Split(resolvedPath, "/")

		for i := len(parts); i > 0; i-- {
			testPath := strings.Join(parts[:i], "/")
			if testPath == from.Pkg {
				return "", false
			}
			// Use the actual rule label from the index — it carries the
			// resolved rule name, which may not match the directory basename
			// (e.g. ts_library_name = "lib" → //packages/foo:lib, not
			// //packages/foo).
			if found := ix.FindRulesByImportWithConfig(nil, resolve.ImportSpec{Lang: languageName, Imp: testPath}, languageName); len(found) > 0 {
				return found[0].Label.Rel(from.Repo, from.Pkg).String(), false
			}
			if found := ix.FindRulesByImportWithConfig(nil, resolve.ImportSpec{Lang: languageName, Imp: testPath + "/*"}, languageName); len(found) > 0 {
				return found[0].Label.Rel(from.Repo, from.Pkg).String(), false
			}
		}
	}
	return "", false
}

// matchNpmPackage returns the package name (handling `@scope/name` correctly)
// if it appears in the package.json deps, else "".
func matchNpmPackage(importPath string, deps map[string]bool) string {
	var pkgName string
	if strings.HasPrefix(importPath, "@") {
		parts := strings.SplitN(importPath, "/", 3)
		if len(parts) < 2 {
			return ""
		}
		pkgName = parts[0] + "/" + parts[1]
	} else {
		parts := strings.SplitN(importPath, "/", 2)
		pkgName = parts[0]
	}
	if deps[pkgName] {
		return pkgName
	}
	// Fallback: type-only imports may resolve to @types/<pkg>.
	if !strings.HasPrefix(pkgName, "@") {
		if typesName := "@types/" + pkgName; deps[typesName] {
			return typesName
		}
	}
	return ""
}

// typesPackageFor returns the @types/* package name paired with `pkgName` if
// one is present in deps, else "".
func typesPackageFor(pkgName string, deps map[string]bool) string {
	var typesName string
	if strings.HasPrefix(pkgName, "@") {
		// Scoped packages get encoded as @types/<scope>__<name> by DefinitelyTyped.
		typesName = "@types/" + strings.Replace(strings.TrimPrefix(pkgName, "@"), "/", "__", 1)
	} else {
		typesName = "@types/" + pkgName
	}
	if deps[typesName] {
		return typesName
	}
	return ""
}

// npmLabel renders the npm-package label using the configured pattern.
func npmLabel(cfg *tsConfig, pkgName string) string {
	return strings.ReplaceAll(cfg.npmLinkPattern, "{pkg}", pkgName)
}

// loadPackageJSONDeps reads the root package.json and seeds packageDeps and
// subpathImportsMap. Idempotent — calling repeatedly is safe.
func (l *tsLang) loadPackageJSONDeps(repoRoot string) {
	if len(l.packageDeps) > 0 {
		return
	}

	data, err := os.ReadFile(repoRoot + "/package.json")
	if err != nil {
		return
	}
	var pkg packageJSON
	if err := json.Unmarshal(data, &pkg); err != nil {
		return
	}
	for dep := range pkg.Dependencies {
		l.packageDeps[dep] = true
	}
	for dep := range pkg.DevDependencies {
		l.packageDeps[dep] = true
	}
	for dep := range pkg.OptionalDependencies {
		l.packageDeps[dep] = true
	}
	for k, v := range pkg.Imports {
		l.subpathImportsMap[k] = v
	}
}

// deduplicateAndSort returns a sorted unique copy of items.
func deduplicateAndSort(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(items))
	out := make([]string, 0, len(items))
	for _, it := range items {
		if !seen[it] {
			seen[it] = true
			out = append(out, it)
		}
	}
	sort.Strings(out)
	return out
}
