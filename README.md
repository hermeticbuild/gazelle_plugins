# gazelle_plugins

Bazel build setup, a Gazelle TypeScript language extension, and the Rust subprocess that powers it.

Built on **Bazel 9 (bzlmod)** with [`rules_rs`](https://github.com/dzbarsky/rules_rs) for the Rust side and `aspect_rules_ts` / `aspect_rules_js` for the TypeScript examples.

## Layout

```
crates/
└── import-extractor/         # Rust subprocess: TS + Python import extraction
                              # (oxc + ruff via length-prefixed protobuf)
gazelle_ts_plugin/             # Go-based Gazelle language extension that emits
                              # stock ts_project / js_test rules
examples/                      # self-contained Bazel workspaces:
├── basic/                    #   one TS package, npm deps, .ts + .tsx + smoke test
├── composite/                #   multi-package with #packages/* cross-refs
├── graphql/                  #   @graphql-codegen → npm_package → composite app
└── advanced/                 #   composite + Bazel-built synthetic npm_package
```

## What this repo gives you

- **`gazelle_ts_plugin`** — generates and maintains `BUILD.bazel` files for TypeScript packages. Emits stock `ts_project` (rules_ts) and `js_test` (rules_js) rules; consumers swap to their own macros via `# gazelle:map_kind`. Reads `package.json` `imports` for subpath resolution. See [`gazelle_ts_plugin/README.md`](gazelle_ts_plugin/README.md).
- **`crates/import-extractor`** — long-lived Rust subprocess that the gazelle plugin spawns for parsing. TypeScript via `oxc`, Python via `ruff`. Length-prefixed protobuf wire protocol over stdin/stdout. See [`crates/import-extractor/README.md`](crates/import-extractor/README.md).
- **`examples/`** — four escalating example workspaces, each with its own `MODULE.bazel`, `pnpm-lock.yaml`, and `tsconfig.json`. They `local_path_override` the parent module so plugin changes apply on the next `bazel run //:gazelle`. See [`examples/README.md`](examples/README.md).

## Build

```
bazel test //...
```

Tests in `crates/` and `gazelle_ts_plugin/` run on linux-x86_64 + macos-arm64 in CI; the example workspaces run on linux-x86_64 only (BUILD-generation coverage doesn't need cross-platform).
