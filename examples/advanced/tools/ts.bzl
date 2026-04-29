"""Project-specific wrappers around stock rules_ts/rules_js.

Gazelle is told to emit these via `# gazelle:map_kind` directives in
//BUILD.bazel:

    # gazelle:map_kind ts_library  myorg_ts_library //tools:ts.bzl
    # gazelle:map_kind ts_test     vitest_test      //tools:ts.bzl
    # gazelle:map_kind js_binary   myorg_js_binary  //tools:ts.bzl

The plugin emits `ts_library` and `ts_test` as abstract kinds with no
compile flags or entry_point — those defaults belong here.

Note: rules_js's `@npm//:vitest/package_json.bzl` re-exports the auto-
generated bin macros under a single `bin = struct(...)` symbol — so the
inner `vitest_test` isn't directly load()-able from there. We re-bind it
here so map_kind can target a single load path.
"""

load("@aspect_rules_js//js:defs.bzl", _js_binary = "js_binary")
load("@aspect_rules_ts//ts:defs.bzl", _ts_project = "ts_project")
load("@npm//:vitest/package_json.bzl", _vitest_bin = "bin")

def myorg_ts_library(name, srcs, **kwargs):
    """Project defaults baked in: shared tsconfig + project-references
    compile flags. The plugin doesn't emit these on ts_library."""
    _ts_project(
        name = name,
        srcs = srcs,
        composite = True,
        declaration = True,
        declaration_map = True,
        source_map = True,
        tsconfig = "//:tsconfig",
        **kwargs
    )

def myorg_js_binary(name, **kwargs):
    """Thin wrapper over js_binary. A real consumer would set default
    NODE_OPTIONS, a wrapping launcher script, or whatever house style
    the binaries should run with."""
    _js_binary(name = name, **kwargs)

# vitest_test auto-discovers test files via the runner's own config — it
# accepts `data` (the test sources, lib outputs, npm packages) without
# needing entry_point. The plugin's ts_test kind shape matches exactly.
vitest_test = _vitest_bin.vitest_test
