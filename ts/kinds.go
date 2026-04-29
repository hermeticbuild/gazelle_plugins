package ts

import (
	"github.com/bazelbuild/bazel-gazelle/rule"
)

// tsKinds describes how Gazelle's merge engine should reconcile our generated
// rules with existing BUILD-file content.
//
//   - NonEmptyAttrs:  attrs that must be non-empty for the rule to survive merge
//   - MergeableAttrs: attrs whose values are merged (union); `# keep` lines
//     in the existing BUILD file are preserved across regenerations
//   - ResolveAttrs:   attrs that are set by Resolve() and replace existing values
//
// Attrs not listed are left untouched if we don't set them, or overwritten if
// we do. This is how manually-set attrs like tsconfig overrides, transpiler
// choices, or custom args survive gazelle runs.
//
// `ts_library`, `ts_test`, and `ts_bundler_config` are all abstract kinds —
// the plugin emits them with the @gazelle_ts//ts:defs.bzl load path, and
// consumers map_kind each to a project-specific macro:
//
//	# gazelle:map_kind ts_library         <macro> <load_path>
//	# gazelle:map_kind ts_test            <macro> <load_path>
//	# gazelle:map_kind ts_bundler_config  <macro> <load_path>
//
// `ts_test` assumes a multi-entry runner (vitest, jest, mocha): it's
// emitted with `data` only, no `entry_point`. Wrappers can pick an entry
// from data when needed by an underlying runner like js_test.
//
// js_binary is hand-written by the user (we never generate it). The plugin
// only fills in `data` based on what its entry_point/srcs import.
const KindJsBinary = "js_binary"

// KindBundlerConfig is the rule emitted for files matched by the
// ts_bundler_config_pattern directive — a separate compilation unit so
// bundler/tooling deps stay out of the library's runtime closure. Abstract
// kind: consumers map_kind it to a real macro.
const KindBundlerConfig = "ts_bundler_config"

var tsKinds = map[string]rule.KindInfo{
	KindTsLibrary: {
		NonEmptyAttrs:  map[string]bool{"name": true},
		MergeableAttrs: map[string]bool{"srcs": true},
		ResolveAttrs: map[string]bool{
			"deps": true,
		},
	},
	KindTsTest: {
		NonEmptyAttrs:  map[string]bool{"name": true},
		MergeableAttrs: map[string]bool{"data": true},
		ResolveAttrs: map[string]bool{
			"data": true,
		},
	},
	KindJsBinary: {
		NonEmptyAttrs:  map[string]bool{"name": true},
		MergeableAttrs: map[string]bool{"data": true},
		ResolveAttrs: map[string]bool{
			"data": true,
		},
	},
	KindBundlerConfig: {
		NonEmptyAttrs:  map[string]bool{"name": true},
		MergeableAttrs: map[string]bool{"srcs": true},
		ResolveAttrs: map[string]bool{
			"deps": true,
		},
	},
}

// Kinds tells Gazelle which rule types this plugin manages.
func (l *tsLang) Kinds() map[string]rule.KindInfo {
	return tsKinds
}

// Loads declares the .bzl files that provide the rule kinds we generate.
// Gazelle uses these to add `load()` statements at the top of BUILD files.
//
// When `# gazelle:map_kind` rewrites our kind to a custom one, the consumer
// is responsible for ensuring the appropriate load statement exists (gazelle
// looks up by the post-map kind, not by the entries here).
func (l *tsLang) Loads() []rule.LoadInfo {
	return []rule.LoadInfo{
		{
			Name:    "@aspect_rules_js//js:defs.bzl",
			Symbols: []string{KindJsBinary},
		},
		{
			Name:    "@gazelle_ts//ts:defs.bzl",
			Symbols: []string{KindTsLibrary, KindTsTest, KindBundlerConfig},
		},
	}
}
