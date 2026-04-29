"""Stubs for the abstract rule kinds emitted by the gazelle_ts plugin.

`ts_library`, `ts_test`, and `ts_bundler_config` are intentionally
abstract: the plugin emits them with this load path, and the consumer
must override each with a project-specific macro via `# gazelle:map_kind`.
The wrappers typically forward to `ts_project` (or `js_test`/`vitest_test`/
`jest_test`) with project-specific defaults baked in (transpiler, tsconfig,
visibility, project-references compile flags, entry_point handling).

The gazelle_ts module deliberately does not take a transitive
`aspect_rules_ts` or `aspect_rules_js` dependency, so the macros can't
live here.

The fallbacks below collect matched files into a filegroup with a debug
print so the BUILD still loads and gazelle can run — but the files are
NOT typechecked or executed. To get a real compilation/test unit, add to
your root BUILD:

    # gazelle:map_kind ts_library         <macro> <load_path>
    # gazelle:map_kind ts_test            <macro> <load_path>
    # gazelle:map_kind ts_bundler_config  <macro> <load_path>

and re-run gazelle. See `examples/basic/tools/library.bzl` and friends
for one-line wrappers.
"""

def _abstract_kind(kind, name, srcs_or_data):
    # buildifier: disable=print
    print(
        kind + "('" + name + "') is using the abstract-kind fallback (no typecheck/execute). " +
        "Add `# gazelle:map_kind " + kind + " <macro> <load_path>` and re-run gazelle.",
    )
    native.filegroup(name = name, srcs = srcs_or_data)

def ts_library(name, srcs, **_kwargs):
    _abstract_kind("ts_library", name, srcs)

def ts_test(name, data, **_kwargs):
    _abstract_kind("ts_test", name, data)

def ts_bundler_config(name, srcs, **_kwargs):
    _abstract_kind("ts_bundler_config", name, srcs)
