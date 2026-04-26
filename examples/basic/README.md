# examples/basic

The smallest possible setup demonstrating the `ts` plugin: one TypeScript package, third-party npm deps, a smoke test. **No internal cross-package references.**

## Layout

```
.
├── app.ts          # imports lodash-es, react-intl, node:path → builds via tsc
├── app.test.ts     # runs via Node 22's --experimental-transform-types
├── package.json    # lodash-es, react-intl + @types/lodash-es, @types/node
└── tsconfig.json
```

## What this verifies

- `ts_project` rule generated with `srcs`, `tsconfig`, `composite`/`declaration`/`source_map`.
- npm packages resolved from `package.json` via `//:node_modules/<pkg>`; `@types/lodash` auto-paired with `lodash`.
- Node.js `node:path` resolved to `@types/node`.
- `js_test` runs the `.ts` file directly under Node 22's type stripping.

## Try it

```bash
bazel run //:gazelle    # generate/update BUILD files
bazel build //...        # compile via tsc
bazel test //...         # run the smoke test
```
