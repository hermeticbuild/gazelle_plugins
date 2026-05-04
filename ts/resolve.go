package ts

import (
	"bytes"
	"encoding/json"
	"log"
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
	Imports              map[string]json.RawMessage `json:"imports"`
	Dependencies         map[string]string          `json:"dependencies"`
	DevDependencies      map[string]string          `json:"devDependencies"`
	OptionalDependencies map[string]string          `json:"optionalDependencies"`
}

var packageImportConditions = map[string]bool{
	"types":       true,
	"node-addons": true,
	"node":        true,
	"import":      true,
	"module-sync": true,
	"default":     true,
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
	case KindTsLibrary:
		// The ts_library wrapper is expected to forward `deps` to its
		// underlying ts_project (or equivalent). One attr for both npm
		// packages and intra-repo project references.
		resolved := l.resolveImportsToDeps(c, importData.Imports, from, ix, cfg)
		all := append([]string{}, resolved.external...)
		all = append(all, resolved.internal...)
		setOrDelete(r, "deps", all)

	case KindTsTest:
		// ts_test uses `data` for everything: every npm package, every
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

	case KindJsBinary, KindTsBinary:
		// We don't generate binary rules — only fill in their `data` attr
		// based on what their entry_point/srcs import. The user's existing
		// entry_point, env, fixed_args, etc. are left alone. Same shape
		// for both stock js_binary and the abstract ts_binary.
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
		capture, ok := matchSubpathImportPattern(pattern, importPath)
		if !ok {
			continue
		}

		for _, target := range l.subpathImportsMap[pattern] {
			if dep, external, ok := l.resolveSubpathTarget(target, capture, from, ix); ok {
				return dep, external
			}
		}
	}
	return "", false
}

func matchSubpathImportPattern(pattern, importPath string) (string, bool) {
	if !strings.Contains(pattern, "*") {
		return "", importPath == pattern
	}

	parts := strings.Split(pattern, "*")
	if len(parts) != 2 {
		return "", false
	}
	prefix, suffix := parts[0], parts[1]
	if !strings.HasPrefix(importPath, prefix) || !strings.HasSuffix(importPath, suffix) {
		return "", false
	}
	captureEnd := len(importPath) - len(suffix)
	if captureEnd < len(prefix) {
		return "", false
	}
	return importPath[len(prefix):captureEnd], true
}

func (l *tsLang) resolveSubpathTarget(target, capture string, from label.Label, ix *resolve.RuleIndex) (string, bool, bool) {
	target = strings.ReplaceAll(target, "*", capture)

	// If the target itself is already a Bazel label, use it directly
	// (with the `*` substituted for the matched suffix). Literal labels
	// are treated as external deps — typical use is wiring a synthetic
	// npm-style package (npm_package, js_library) for things not in
	// package.json.
	if strings.HasPrefix(target, "//") || strings.HasPrefix(target, "@") {
		dep, err := label.Parse(target)
		if err != nil {
			log.Fatalf("package.json imports target %q expanded to invalid label: %v", target, err)
		}
		return dep.Rel(from.Repo, from.Pkg).String(), true, true
	}
	if ix == nil {
		return "", false, false
	}

	// Otherwise treat target as a path within the repo and look up the
	// matching ts_project in the rule index.
	resolvedPath := strings.TrimPrefix(target, "./")
	for _, ext := range []string{".js", ".ts", ".tsx", ".jsx"} {
		resolvedPath = strings.TrimSuffix(resolvedPath, ext)
	}
	parts := strings.Split(resolvedPath, "/")

	for i := len(parts); i > 0; i-- {
		testPath := strings.Join(parts[:i], "/")
		if testPath == from.Pkg {
			return "", false, true
		}
		for _, imp := range []string{testPath, testPath + "/*"} {
			found := ix.FindRulesByImportWithConfig(nil, resolve.ImportSpec{Lang: languageName, Imp: imp}, languageName)
			sort.Slice(found, func(i, j int) bool {
				return len(found[i].Label.Pkg) > len(found[j].Label.Pkg)
			})
			for _, candidate := range found {
				if candidate.Label.Pkg == from.Pkg {
					return "", false, true
				}
				// Use the actual rule label from the index — it carries the
				// resolved rule name, which may not match the directory basename
				// (e.g. ts_library_name = "lib" → //packages/foo:lib, not
				// //packages/foo).
				return candidate.Label.Rel(from.Repo, from.Pkg).String(), false, true
			}
		}
	}

	return "", false, false
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
	for k, raw := range pkg.Imports {
		if targets := decodePackageImportTargets(raw); len(targets) > 0 {
			l.subpathImportsMap[k] = targets
		}
	}
}

func decodePackageImportTargets(raw json.RawMessage) []string {
	if string(bytes.TrimSpace(raw)) == "null" {
		return nil
	}

	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		if s == "" {
			return nil
		}
		return []string{s}
	}

	var arr []json.RawMessage
	if err := json.Unmarshal(raw, &arr); err == nil {
		var out []string
		for _, item := range arr {
			out = append(out, decodePackageImportTargets(item)...)
		}
		return out
	}

	if entries, ok := decodeJSONObjectEntries(raw); ok {
		for _, entry := range entries {
			if !packageImportConditions[entry.key] {
				continue
			}
			if targets := decodePackageImportTargets(entry.value); len(targets) > 0 {
				return targets
			}
		}
	}

	return nil
}

type jsonObjectEntry struct {
	key   string
	value json.RawMessage
}

func decodeJSONObjectEntries(raw json.RawMessage) ([]jsonObjectEntry, bool) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	tok, err := dec.Token()
	if err != nil {
		return nil, false
	}
	delim, ok := tok.(json.Delim)
	if !ok || delim != '{' {
		return nil, false
	}

	var entries []jsonObjectEntry
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return nil, false
		}
		key, ok := tok.(string)
		if !ok {
			return nil, false
		}
		var value json.RawMessage
		if err := dec.Decode(&value); err != nil {
			return nil, false
		}
		entries = append(entries, jsonObjectEntry{key: key, value: value})
	}

	tok, err = dec.Token()
	if err != nil {
		return nil, false
	}
	delim, ok = tok.(json.Delim)
	if !ok || delim != '}' {
		return nil, false
	}
	return entries, true
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
