"""Concrete macros for the abstract kinds emitted by gazelle_ts.

ts_library / ts_test / ts_bundler_config all forward to ts_project /
js_test with the project's defaults baked in.
"""

load("@aspect_rules_ts//ts:defs.bzl", "ts_project")
load("@aspect_rules_js//js:defs.bzl", "js_test")

def _project(name, srcs, **kwargs):
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

def ts_library(name, srcs, **kwargs):
    _project(name = name, srcs = srcs, **kwargs)

def ts_bundler_config(name, srcs, **kwargs):
    _project(name = name, srcs = srcs, **kwargs)

def ts_test(name, data, **kwargs):
    # Pick the first .test.ts* in data as entry_point for stock js_test.
    entry = None
    for d in data:
        if d.endswith(".test.ts") or d.endswith(".test.tsx"):
            entry = d
            break
    js_test(name = name, data = data, entry_point = entry, **kwargs)
