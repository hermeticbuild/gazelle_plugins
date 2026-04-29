"""Stub for the `ts_bundler_config` rule emitted by the gazelle plugin.

`ts_bundler_config` is intentionally an abstract kind: the plugin emits the
rule with this load path, but the consumer is expected to override it with
a project-specific macro via `# gazelle:map_kind`. The macro typically
wraps `ts_project` with whatever transpiler / tsconfig / visibility shape
fits the consumer workspace; the gazelle_ts module deliberately does not
take a transitive `aspect_rules_ts` dependency, so the macro can't live
here.

The fallback below collects matched files into a filegroup so the BUILD
still loads and gazelle can run — but the bundler-config files are NOT
typechecked. To get a real compilation unit, add to your root BUILD:

    # gazelle:map_kind ts_bundler_config <your_macro> <your_load_path>

and re-run gazelle. See `examples/bundler-config/tools/bundler.bzl` for
a one-line wrapper over ts_project.
"""

def ts_bundler_config(name, srcs, **_kwargs):
    # buildifier: disable=print
    print(
        "ts_bundler_config('" + name + "') is using the abstract-kind fallback (no typecheck). " +
        "Add `# gazelle:map_kind ts_bundler_config <macro> <load_path>` and re-run gazelle.",
    )
    native.filegroup(name = name, srcs = srcs)
