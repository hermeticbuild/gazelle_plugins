"""Concrete macro for ts_bundler_config.

The gazelle_ts plugin emits the abstract `ts_bundler_config` kind for files
matched by `ts_bundler_config_pattern`. We rewrite that kind to this macro
via `# gazelle:map_kind ts_bundler_config bundler_config //tools:bundler.bzl`
in the root BUILD. The wrapping is paper-thin: forward every gazelle-set
attr to ts_project so the shared tsconfig (composite, declaration,
sourceMap) lines up with the rule's attrs.
"""

load("@aspect_rules_ts//ts:defs.bzl", "ts_project")

def bundler_config(name, srcs, **kwargs):
    ts_project(
        name = name,
        srcs = srcs,
        **kwargs
    )
