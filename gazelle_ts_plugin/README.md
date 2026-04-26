# gazelle_ts_plugin

A Gazelle language extension that generates and maintains BUILD files for TypeScript packages. It emits stock [`rules_ts`](https://github.com/aspect-build/rules_ts) and [`rules_js`](https://github.com/aspect-build/rules_js) rules:

- `ts_project` for libraries (from `@aspect_rules_ts//ts:defs.bzl`)
- `js_test` for tests (from `@aspect_rules_js//js:defs.bzl`)

If you have your own TypeScript / test macros, use Gazelle's `# gazelle:map_kind` directive to swap the emitted kinds:

```starlark
# gazelle:map_kind ts_project myrepo_ts_library //tools:ts.bzl
# gazelle:map_kind js_test    myrepo_ts_test    //tools:ts.bzl
```

The plugin is the same; only the kind name and load path on the resulting rule change. Note that `map_kind` doesn't rewrite attribute names — your macro must accept the attrs we set (see [Generated attrs](#generated-attrs) below).

## Quickstart

Add a `BUILD.bazel` at the repo root with:

```starlark
load("@gazelle//:def.bzl", "gazelle")

gazelle(
    name = "gazelle",
    gazelle = "@//gazelle_ts_plugin:gazelle_ts",
)
```

Then run `bazel run //:gazelle`.

## Directives

All directives are placed in `BUILD.bazel` as `# gazelle:<key> <value>` and inherit into subdirectories.

| Directive | Default | Notes |
|---|---|---|
| `ts_enabled` | `true` | Disable per-tree to skip directories owned by another tool. |
| `ts_library_name` | `lib` | Name of the generated library rule. |
| `ts_test_name` | `test` | Name of the generated test rule. |
| `ts_library_kind` | `ts_project` | Override emitted library kind without `map_kind`. |
| `ts_test_kind` | `js_test` | Override emitted test kind without `map_kind`. |
| `ts_visibility` | `//visibility:public` | Repeatable / space-separated list. |
| `ts_test_pattern` | `*.test.ts`, `*.test.tsx`, `tests/**`, `test/**` | Repeatable; appended. |
| `ts_extension` | `.ts`, `.tsx` | Repeatable; appended. |
| `ts_project_references` | `true` | Emits `composite = True` and the resolved `references` attr. |
| `ts_tsconfig` | _(unset)_ | Set to a Bazel label to emit `tsconfig` on every library. |
| `ts_npm_link_pattern` | `//:node_modules/{pkg}` | Template; `{pkg}` is replaced with the resolved package name. |
| `ts_generated_package` | _(from `package.json` `imports`)_ | Repeatable `pattern=target` entries; maps a generated/synthetic package namespace to a Bazel label. Merged on top of `package.json`. |
| `ts_test_data` | _(empty)_ | Repeatable; appended to every test rule's `data`. |
| `ts_test_entry_point` | _first matching `*.test.ts*`_ | Override the entry point picked for tests. |

### `ts_generated_package` examples

```
# Map @myrepo_generated/* directly to a Bazel label
# gazelle:ts_generated_package @myrepo_generated/*=//:node_modules/@myrepo_generated/*

# Map a Node.js subpath import (#packages/*) to a workspace path
# gazelle:ts_generated_package #packages/*=./packages/*
```

The first form (target starts with `//` or `@`) is taken as a Bazel label literal. The second form (relative path) is treated as a workspace path; the plugin walks the rule index to find the longest matching package.

## Generated attrs

### `ts_project`

| Attr | Set by | Behavior |
|---|---|---|
| `name` | generate | non-empty required |
| `srcs` | generate | mergeable, preserves `# keep` lines |
| `visibility` | generate | overwritten each run |
| `deps` | resolve | replaced each run |
| `references` | resolve | replaced when `ts_project_references = true` |
| `composite`, `declaration`, `source_map` | generate | only when `ts_project_references = true` |
| `tsconfig` | generate | only when `ts_tsconfig` directive is set |
| anything else | _untouched_ | manual overrides survive across runs |

### `js_test`

| Attr | Set by | Behavior |
|---|---|---|
| `name` | generate | non-empty required |
| `srcs` | generate | mergeable |
| `data` | generate | mergeable |
| `deps` | resolve | replaced each run |
| `entry_point` | generate | from `ts_test_entry_point` or first `*.test.ts*` |
| anything else | _untouched_ | |

## How import resolution works

1. `package.json` is read once at the repo root for `dependencies` / `devDependencies` / `optionalDependencies` / `imports`.
2. Imports are categorized:
   - Relative (`./foo`, `../bar`): no dep added.
   - Subpath (matches a key in `subpathImportsMap`): resolves to an internal repo label.
   - Node.js builtin: resolves to `@types/node`.
   - npm package: resolves to `{npmLinkPattern}` with `{pkg}` replaced; auto-pairs `@types/<pkg>` if present in deps.
3. Library rules get `deps` (npm) and `references` (internal). Test rules collapse both into `deps`.

## Architecture

The plugin spawns the Rust `import-extractor` subprocess (built from `//crates/import-extractor:bin`) once per Gazelle run and streams length-prefixed protobuf frames to it for parsing. See [`crates/import-extractor/README.md`](../crates/import-extractor/README.md) for the wire-protocol details.
