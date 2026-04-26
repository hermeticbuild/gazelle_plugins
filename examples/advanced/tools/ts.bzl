"""Project-specific wrappers around stock rules_ts/rules_js.

Gazelle is told to emit these via `# gazelle:map_kind` directives in
//BUILD.bazel:

    # gazelle:map_kind ts_project myorg_ts_library //tools:ts.bzl
    # gazelle:map_kind js_test    vitest_test     //tools:ts.bzl

The plugin still computes everything (srcs, deps, composite, etc.) — these
wrappers just give you a place to bake in project defaults (a shared
tsconfig, an opinionated transpiler, vitest's own config) without forking
the gazelle plugin.
"""

load("@aspect_rules_js//js:defs.bzl", _js_test = "js_test")
load("@aspect_rules_ts//ts:defs.bzl", _ts_project = "ts_project")

def myorg_ts_library(name, **kwargs):
    """Thin wrapper over ts_project. A real consumer would set a default
    tsconfig, transpiler, or npm packaging metadata here."""
    _ts_project(
        name = name,
        **kwargs
    )

def vitest_test(name, **kwargs):
    """Stand-in for a real vitest_test rule. A real consumer would invoke
    vitest via js_run_devserver / js_test with vitest as entry_point.
    Here we delegate to js_test directly so the plugin's data/entry_point
    attrs flow through unchanged."""
    _js_test(
        name = name,
        **kwargs
    )
