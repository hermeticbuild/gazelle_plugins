"""Public macros for gazelle_ts-emitted rule kinds.

The plugin emits `ts_bundler_config` for files matched by the
`ts_bundler_config_pattern` directive (vite/vitest/tailwind configs etc.).
This macro is the default implementation: a thin wrapper over `ts_project`
with bundler-friendly defaults so the config typechecks as its own
compilation unit but does not contribute to the lib's runtime closure.

Consumers who want a different shape (e.g. a project-specific transpiler
or no typecheck at all) should `# gazelle:map_kind ts_bundler_config
<their_macro> //path/to:their.bzl` to swap. The plugin emits the kind name
unchanged; gazelle rewrites the load statement on disk.
"""

load("@aspect_rules_ts//ts:defs.bzl", "ts_project")

def ts_bundler_config(name, srcs, deps = None, tsconfig = None, visibility = None, **kwargs):
    """Wraps ts_project for bundler/tooling config files.

    Defaults:
      - composite / declaration / source_map: False (bundler configs are
        leaves, not project references; declarations would pull tsc into
        the lib's reference graph which defeats the boundary).
      - visibility: package-private (configs are not meant to be imported
        across packages; if you need cross-package use, set visibility
        explicitly).

    Anything not covered by the defaults forwards to ts_project — set
    `transpiler`, `tsconfig`, `args`, `data`, etc. as needed.
    """
    ts_project(
        name = name,
        srcs = srcs,
        deps = deps or [],
        tsconfig = tsconfig,
        visibility = visibility or ["//visibility:private"],
        composite = False,
        declaration = False,
        source_map = False,
        **kwargs
    )
