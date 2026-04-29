# RFC: `ts_bundler_config_pattern` directive and `ts_bundler_config` rule

Status: Draft
Author: Long Ho
Date: 2026-04-29

## Problem

In TypeScript projects that use Vite, Vitest, Tailwind, or similar tooling, configuration files (`vite.config.ts`, `vitest.config.ts`, `tailwind.config.ts`, …) live in the same directory as the library/app code they configure, but their import closures are fundamentally different:

- The library imports React, the app's own modules, and runtime npm packages.
- The bundler config imports `vite`, `@vitejs/plugin-react`, `vitest`, plugin packages — none of which are runtime deps of the library.

When `gazelle_ts` generates a single `ts_project` per package, the resulting `deps` attribute is the union of both closures. The library target ends up depending on `vite` / `vitest` / etc., which:

1. Bloats the runtime dep graph with build-time-only packages.
2. Breaks hermetic separation — a typecheck or build of the library should not require bundler internals.
3. Allows lib code to silently `import './vite.config'`, dragging the bundler closure back in transitively. Routing the deps to a separate attr alone does not prevent this — only a real compilation boundary does.
4. Forces maintainers to either accept the bloat or move configs into a separate Bazel package, which fights filesystem ergonomics (configs conventionally sit next to the code they configure).

`ts_test_pattern` already peels test files into a separate `js_test` so test-only deps don't pollute the main `ts_project`. Bundler configs need a similar split, but with a stronger requirement: they must be a *separate compilation unit*, not just a separate dep bucket, so accidental coupling between lib and config is caught at typecheck/build time.

## Non-goal: routing-only via `ts_config_file`

A previous in-flight directive, `# gazelle:ts_config_file <glob> <attr>` (commit `b490e97`, branch `ts-config-file-directive`), routes config-file imports to a named attr on the library rule without emitting a separate target. That solves the dep-bloat half but not the boundary-enforcement half: the library and the config still share a compilation unit, so `lib.ts` can import the config and re-introduce the coupling. This RFC supersedes that work; `b490e97` is to be reverted before this lands.

## Proposal

Two pieces, designed together:

### 1. `# gazelle:ts_bundler_config_pattern <glob> <name>`

Repeatable directive. Each entry nominates a single config file (by glob) and the Bazel target name to emit for it.

```python
# gazelle:ts_bundler_config_pattern vite.config.* vite_config
# gazelle:ts_bundler_config_pattern vitest.config.* vitest_config
# gazelle:ts_bundler_config_pattern tailwind.config.ts tailwind_config
# gazelle:ts_bundler_config_pattern .storybook/main.ts storybook_config
```

- Glob syntax matches `ts_test_pattern`'s matcher (`*` segment wildcard, `**` spans directories).
- Pattern is relative to the package directory.
- The `<name>` is the literal Bazel target name. The plugin does not derive names from filenames, so `.config.cjs`, `.mts`, `main.ts`, etc. all work without a special-case rule.
- When multiple specs match the same file, the longest pattern wins. Each matched file produces exactly one `ts_bundler_config` target.

### 2. `ts_bundler_config` rule

A new rule kind emitted by the plugin. The rule is a thin macro shipped from `ts/defs.bzl` (or equivalent) that wraps `ts_project` with bundler-config-friendly defaults:

- No project references.
- No declaration emission.
- Transpile-only typecheck (configurable; default mirrors the lib's transpiler choice).
- Visibility defaults to package-private.

The macro is concrete, not abstract: a fresh `gazelle_ts` consumer can use the directive without writing any wrapping code. Consumers who want different behavior can `map_kind ts_bundler_config <their_macro> //...`, which only rewrites bundler-config emissions and leaves library `ts_project`s untouched. This is the reason for a distinct kind: `ts_project` cannot serve both purposes under `map_kind`.

## Plugin behavior

For each file matching any `ts_bundler_config_pattern`:

1. **Exclude from the main `ts_project`.** The file is removed from the package-level lib target's `srcs`. Its imports are not added to the lib's `deps`.
2. **Exclude from the test target.** A file matching both a test pattern and a bundler-config pattern goes to the bundler-config target, not the test target.
3. **Emit one `ts_bundler_config` rule per matched file**, named per the directive's `<name>` argument. Same import scanner, same resolution path as the main target — only the input file set differs.
4. **Resolve a separate deps closure.** The bundler-config target's `deps` are derived only from the imports of the matched file (and any local helpers it pulls in; see below).
5. **Local helpers stay in the library.** If `vite.config.ts` and `lib.ts` both import `./shared.ts`, `shared.ts` remains in the lib target's `srcs`, and the bundler-config target gets `:<lib_name>` added to its `deps`. The asymmetry — bundler-config closure includes lib, never the reverse — preserves the boundary.
6. **`lib.ts` importing a bundler-config file is a build error.** The resolver leaves the import unresolved; `ts_project` reports it as a missing import. Routing the import to the bundler-config target would silently re-create the coupling and is explicitly rejected.
7. **Stale targets are cleaned up.** When a pattern is removed or renamed, the previously-emitted `ts_bundler_config` rule is dropped from the BUILD file on the next `gazelle` run. Achieved by registering the kind in `KindInfo` with the right merge attrs so gazelle's standard rule reconciliation handles it.

## Symmetry with `ts_test_pattern`

|                | `ts_test_pattern`                                        | `ts_bundler_config_pattern` (proposed)            |
|----------------|----------------------------------------------------------|---------------------------------------------------|
| Emits          | `js_test`                                                | `ts_bundler_config` (new kind)                    |
| Aggregation    | one target per package                                   | one target per matched file                       |
| Deps source    | imports of test files                                    | imports of the matched config file                |
| Why separate   | test-only deps shouldn't enter runtime closure           | bundler-only deps shouldn't enter runtime closure |
| Boundary       | enforced (separate target, separate compile)             | enforced (separate target, separate compile)      |
| Helper rule    | helpers stay in lib                                      | helpers stay in lib                               |

The matcher (`matchTestPattern` in `ts/generate.go`) is reused. Resolve plumbing is extended to support N config-file buckets (one per emitted target) rather than the current two-bucket lib/test split.

## API surface

Directives:

| Directive                           | Default | Meaning                                                                 |
|-------------------------------------|---------|-------------------------------------------------------------------------|
| `ts_bundler_config_pattern <glob> <name>` | _(empty)_ | Repeatable. Each match emits a `ts_bundler_config` target named `<name>`. |

Rule:

| Rule                  | Load from           | Macro defaults                                                  |
|-----------------------|---------------------|-----------------------------------------------------------------|
| `ts_bundler_config`   | `@gazelle_ts//ts:defs.bzl` (TBD path) | wraps `ts_project`; no references, no declarations, package-private |

`KindInfo`:

| Attr        | Behavior                |
|-------------|-------------------------|
| `srcs`      | merge (gazelle-managed) |
| `deps`      | resolve (gazelle-managed) |
| `tsconfig`  | merge (only if `ts_tsconfig` is set) |
| anything else | untouched (manual overrides survive) |

## Worked example

Input directory:

```
app/
├── BUILD.bazel
├── index.ts                # imports react, ./helpers
├── helpers.ts              # imports lodash
├── vite.config.ts          # imports vite, @vitejs/plugin-react, ./viteHelpers
├── viteHelpers.ts          # imports node:path
└── index.test.ts           # imports vitest, ./index
```

`BUILD.bazel`:

```python
# gazelle:ts_bundler_config_pattern vite.config.* vite_config

ts_project(
    name = "app",
    srcs = ["index.ts", "helpers.ts", "viteHelpers.ts"],
    deps = [
        "//:node_modules/react",
        "//:node_modules/lodash",
    ],
)

ts_bundler_config(
    name = "vite_config",
    srcs = ["vite.config.ts"],
    deps = [
        ":app",  # because vite.config.ts imports ./viteHelpers, which lives in lib
        "//:node_modules/vite",
        "//:node_modules/@vitejs/plugin-react",
    ],
)

js_test(
    name = "app_test",
    data = [":app", "//:node_modules/vitest"],
    entry_point = "index.test.ts",
    ...
)
```

If `index.ts` were edited to add `import './vite.config'`, the resolver would leave that import unresolved and the build would fail at typecheck — the boundary is enforced.

## Open questions

1. **`viteHelpers.ts` ownership in the example above.** The proposal puts it in lib because lib never imports it but the bundler config does, so it's only reachable from the bundler-config target. Two reasonable alternatives:
   - Detect "only the config file imports this" and put it in the bundler-config target's `srcs` instead of the lib's. Tighter closure for lib, but requires a reachability pass and a tiebreak when multiple bundler configs share a helper.
   - Always lib (as specified above). Simpler; lib carries a few extra bytes.
   The RFC defaults to "always lib" for simplicity. Revisit if the bloat is measurable.

2. **Macro defaults for `ts_bundler_config`.** "No references, no declarations, package-private" is a starting point. Real-world bundler configs may need `noEmit = True` or a specific tsconfig. Decide whether the macro takes a `tsconfig` attr or inherits from package-level `ts_tsconfig`.

3. **Multiple bundler configs that share a helper.** If `vite.config.ts` and `vitest.config.ts` both import `./shared.ts` and neither lib code imports it, where does `shared.ts` go? Per the "always lib" rule above: lib. Both bundler-config targets depend on `:lib`. Acceptable.

## Out of scope

- Auto-detection of common config filenames without an explicit directive. Users opt in.
- A `ts_bundler_config_name_template` directive — obviated by making the name part of the directive itself.
- Migration tooling for `ts_config_file` users — that directive has not shipped to consumers; reverting `b490e97` is sufficient.

## Migration / rollout

1. Revert `b490e97` (`ts_config_file`) on `main` before this lands.
2. Implement `ts_bundler_config_pattern` directive parsing + `ts_bundler_config` rule + `KindInfo` registration.
3. Ship the `ts_bundler_config` macro in `ts/defs.bzl`.
4. Update `ts/README.md` with the directive table entry and a worked example.
5. Add an example under `examples/` exercising vite + vitest configs in the same package.
