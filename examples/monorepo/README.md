# examples/monorepo

A self-contained Bazel workspace demonstrating `gazelle_ts_plugin` against a representative TypeScript monorepo: two top-level apps, a shared core package, and deeply nested composite utilities.

## Layout

```
.
├── apps/
│   ├── web/       # browser app — React + react-query + cross-package refs
│   └── cli/       # node CLI — node:fs + zod + cross-package refs
└── packages/
    ├── core/                    # base package, no internal deps
    └── utils/
        ├── string/              # depends on packages/core
        └── math/deep/           # depends on packages/core, deeply nested
```

Each `*.ts` file demonstrates a different mix of import patterns — npm packages (bare and scoped), `@types/*` auto-pairing, Node.js builtins, and `package.json`-mapped subpath imports across packages.

## What this verifies

- Cross-package `references` (TS project references) are resolved across arbitrary nesting depth.
- npm packages (including scoped ones) round-trip through `ts_npm_link_pattern`.
- `@types/*` packages are auto-added when the runtime package has them.
- Node.js builtins resolve to `@types/node`.
- Test files (`*.test.ts`) get their own `js_test` rule with `entry_point` set.

## How it builds without a real `rules_ts` setup

This example points `# gazelle:map_kind` at stub `ts_project` / `js_test` macros in [`tools/stubs.bzl`](tools/stubs.bzl), so generated BUILD files load and `bazel build //...` succeeds without pnpm or tsc. Real consumers replace those map_kind directives with `@aspect_rules_ts//ts:defs.bzl` and `@aspect_rules_js//js:defs.bzl` (or their own macros).

`tools/npm/` is a hand-curated set of stub `filegroup` targets that stand in for what `npm_link_all_packages()` would create from a pnpm-lock. Real consumers delete `tools/npm/`.

## Try it

```bash
# Regenerate BUILD files from current source
bazel run //:gazelle

# Build everything
bazel build //...

# Run tests (the stub test runner just exits 0)
bazel test //...
```

## Adapting this to your repo

1. Drop the stub map_kind directives in `BUILD.bazel`.
2. Add `aspect_rules_ts` + `aspect_rules_js` + a pnpm setup to your `MODULE.bazel`.
3. Set `# gazelle:ts_npm_link_pattern //pnpm:node_modules/{pkg}` (or wherever your `npm_link_all_packages` lives).
4. Set `# gazelle:ts_tsconfig //:tsconfig` so generated `ts_project` rules pick up your shared tsconfig.
5. Run `bazel run //:gazelle`.
