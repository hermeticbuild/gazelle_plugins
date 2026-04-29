# examples/bundler-config

Demonstrates the `# gazelle:ts_bundler_config_pattern` directive: peeling bundler/tooling config files (vite, vitest, tailwind) out of the library compilation unit so build-time deps don't enter the runtime closure, and accidental `import './vite.config'` from lib code fails at typecheck.

## Layout

```
.
├── app.ts                  # imports lodash-es + ./viteHelpers
├── app.test.ts             # smoke test under Node 22's transform-types
├── viteHelpers.ts          # shared helper used by lib AND vite.config
├── vite.config.ts          # imports vite, @vitejs/plugin-react, ./viteHelpers
├── vitest.config.ts        # imports vitest/config
├── tailwind.config.ts      # imports tailwindcss types
├── tools/bundler.bzl       # one-line ts_project wrapper for map_kind
└── tsconfig.json
```

## Generated BUILD shape

After `bazel run //:gazelle`:

```python
ts_project(
    name = "lib",
    srcs = ["app.ts", "viteHelpers.ts"],
    deps = ["//:node_modules/@types/lodash-es", "//:node_modules/lodash-es"],
)

bundler_config(             # was ts_bundler_config; rewritten by map_kind
    name = "vite_config",
    srcs = ["vite.config.ts"],
    deps = [
        ":lib",             # because vite.config imports ./viteHelpers, which lives in :lib
        "//:node_modules/@vitejs/plugin-react",
        "//:node_modules/vite",
    ],
)

bundler_config(name = "vitest_config", ...)
bundler_config(name = "tailwind_config", ...)
```

Note: `vite`, `vitest`, `@vitejs/plugin-react`, `tailwindcss` are **not** on `:lib`'s deps. They live exclusively on the bundler-config targets.

## Map_kind is required

`ts_bundler_config` is shipped as an abstract kind — gazelle_ts deliberately doesn't take a transitive `aspect_rules_ts` dependency. Each consumer wires their own concrete macro:

```
# gazelle:map_kind ts_bundler_config bundler_config //tools:bundler.bzl
```

`tools/bundler.bzl` here is a one-liner forwarding to `ts_project`. Real-world consumers customize defaults (transpiler, tsconfig, visibility) per their project.

## Verifying the boundary

Add `import './vite.config.js';` to `app.ts` and run `bazel build //:lib` — tsc fails with `error TS2882: Cannot find module or type declarations for side-effect import of './vite.config.js'`. The resolver does **not** route the import to `:vite_config`, because that would silently re-create the coupling the directive exists to prevent.

## Try it

```bash
bazel run //:gazelle    # generate/update BUILD files
bazel build //...        # compile lib + bundler configs via tsc
bazel test //...         # run the smoke test
```
