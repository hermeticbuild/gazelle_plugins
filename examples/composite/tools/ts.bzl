"""Concrete macros for the abstract kinds emitted by gazelle_ts."""

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
    entry = None
    for d in data:
        if d.endswith(".test.ts") or d.endswith(".test.tsx"):
            entry = d
            break
    js_test(name = name, data = data, entry_point = entry, **kwargs)
