"""Concrete macros for the abstract kinds emitted by gazelle_ts.

The plugin emits ts_library / ts_test / ts_bundler_config — abstract kinds
that we map_kind here to ts_project / js_test calls with the project's
defaults baked in.
"""

load("@aspect_rules_ts//ts:defs.bzl", "ts_project")
load("@aspect_rules_js//js:defs.bzl", "js_test")

def ts_library(name, srcs, tsconfig_types = None, **kwargs):
    ts_project(
        name = name,
        srcs = srcs,
        composite = True,
        declaration = True,
        declaration_map = True,
        source_map = True,
        tsconfig = "//:tsconfig",
        **kwargs
    )

def ts_test(name, data, tsconfig_types = None, **kwargs):
    # Stock js_test needs an entry_point; pick the first .test.ts* in data.
    # If you migrate to vitest_test/jest_test, drop this and forward data
    # straight to the runner.
    entry = None
    for d in data:
        if d.endswith(".test.ts") or d.endswith(".test.tsx"):
            entry = d
            break
    js_test(name = name, data = data, entry_point = entry, **kwargs)
