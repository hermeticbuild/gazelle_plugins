"""Stub implementations of ts_project and js_test for the example.

The example doesn't pull in aspect_rules_ts / aspect_rules_js — that would
require a real pnpm/tsc setup. Instead we expose stubs with the same attr
surface so generated BUILD files load and `bazel build //...` succeeds. The
point of this example is to exercise the gazelle plugin's BUILD generation,
not to actually compile TypeScript.

In a real consumer, swap these out:

    # gazelle:map_kind ts_project ts_project @aspect_rules_ts//ts:defs.bzl
    # gazelle:map_kind js_test    js_test    @aspect_rules_js//js:defs.bzl
"""

load("@rules_shell//shell:sh_test.bzl", "sh_test")

def ts_project(
        name,
        srcs = [],
        deps = [],
        references = [],
        composite = False,
        declaration = False,
        source_map = False,
        tsconfig = None,
        visibility = None,
        **kwargs):
    """Stub: produces a filegroup so cross-package deps resolve."""
    _ = composite
    _ = declaration
    _ = source_map
    _ = tsconfig
    _ = kwargs
    native.filegroup(
        name = name,
        srcs = srcs + deps + references,
        visibility = visibility,
    )

def js_test(
        name,
        srcs = [],
        deps = [],
        data = [],
        entry_point = None,
        visibility = None,
        **kwargs):
    """Stub: produces a no-op sh_test that always passes."""
    _ = entry_point
    _ = kwargs
    sh_test(
        name = name,
        srcs = ["//tools:noop_test.sh"],
        data = srcs + deps + data,
        visibility = visibility,
    )
