# examples/composite

Multiple TypeScript packages with cross-package references. Demonstrates `package.json` subpath imports (`#packages/*`) resolving to internal Bazel targets and TypeScript project references being wired through `composite = True` + `deps`.

## Layout

```
.
├── apps/
│   └── web/
│       ├── UserList.tsx       # imports from #packages/core, #packages/utils
│       └── UserList.test.ts   # smoke test
└── packages/
    ├── core/
    │   └── types.ts           # base User type, no internal deps
    └── utils/
        └── format.ts          # depends on #packages/core
```

## What this verifies

- `# gazelle:ts_npm_link_pattern` directive applied repo-wide; tsconfig + project-references compile flags are baked into the `ts_library` wrapper at `tools/ts.bzl` (the plugin no longer emits these attrs directly).
- Cross-package imports via `package.json` `"imports": { "#packages/*": "./packages/*" }` resolve to `//packages/<name>` Bazel targets.
- TS project references work — every `ts_library` wrapper sets `composite = True`, with `deps` carrying the cross-package labels.
- npm packages and `@types/*` auto-pairing work alongside internal deps.

## Try it

```bash
bazel run //:gazelle    # generate/update BUILD files in every package dir
bazel build //...        # compiles every package + its references
bazel test //...         # runs the smoke tests
```
