"""Project-specific wrappers around stock rules_ts/rules_js.

Gazelle is told to emit these via `# gazelle:map_kind` directives in
//BUILD.bazel:

    # gazelle:map_kind ts_project myorg_ts_library //tools:ts.bzl
    # gazelle:map_kind js_test    vitest_test     //tools:ts.bzl

Note: rules_js's `@npm//:vitest/package_json.bzl` re-exports the auto-
generated bin macros under a single `bin = struct(...)` symbol — so the
inner `vitest_test` isn't directly load()-able from there. We re-bind it
here so map_kind can target a single load path.
"""

load("@aspect_rules_ts//ts:defs.bzl", _ts_project = "ts_project")
load("@npm//:vitest/package_json.bzl", _vitest_bin = "bin")

def myorg_ts_library(name, **kwargs):
    """Thin wrapper over ts_project. A real consumer would set a default
    tsconfig, transpiler, or npm packaging metadata here."""
    _ts_project(name = name, **kwargs)

vitest_test = _vitest_bin.vitest_test
