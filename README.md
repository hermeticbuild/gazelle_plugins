# gazelle_ts

Bazel build setup, a Gazelle TypeScript language extension, and the Rust subprocess that powers it.

Built on **Bazel 9 (bzlmod)** with [`rules_rs`](https://github.com/dzbarsky/rules_rs) for the Rust side and `aspect_rules_ts` / `aspect_rules_js` for the TypeScript examples.

## Layout

```
crates/
└── import_extractor/         # Rust subprocess: TS import extraction
                              # (oxc via length-prefixed protobuf)
ts/                           # Go-based Gazelle language extension that emits
                              # stock ts_project / js_test rules
examples/                      # self-contained Bazel workspaces:
├── basic/                    #   one TS package, npm deps, .ts + .tsx + smoke test
├── composite/                #   multi-package with #packages/* cross-refs
├── graphql/                  #   @graphql-codegen → npm_package → composite app
└── advanced/                 #   composite + Bazel-built synthetic npm_package
```

## What this repo gives you

- **`ts`** — Gazelle TypeScript Language extension. Generates and maintains `BUILD.bazel` files for TypeScript packages, emitting stock `ts_project` (rules_ts) and `js_test` (rules_js) rules; consumers swap to their own macros via `# gazelle:map_kind`. Reads `package.json` `imports` for subpath resolution. Consume by composing your own `gazelle_binary(languages = ["@gazelle_ts//ts"])`. See [`ts/README.md`](ts/README.md).
- **`crates/import_extractor`** — long-lived Rust subprocess that the gazelle plugin spawns for parsing. TypeScript via `oxc`. Length-prefixed protobuf wire protocol over stdin/stdout. See [`crates/import_extractor/README.md`](crates/import_extractor/README.md).
- **`import_extractor/`** — module extension that downloads the pre-built import-extractor binary from a GitHub release. Configurable version + per-platform URL/SHA256 overrides; modeled on rules_formatjs's pattern. See [`import_extractor/README.md`](import_extractor/README.md).
- **`examples/`** — four escalating example workspaces, each with its own `MODULE.bazel`, `pnpm-lock.yaml`, and `tsconfig.json`. They `local_path_override` the parent module so plugin changes apply on the next `bazel run //:gazelle`. See [`examples/README.md`](examples/README.md).

## Build

```
bazel test //...
```

Tests in `crates/` and `ts/` run on linux-x86_64 + macos-arm64 in CI; the example workspaces run on linux-x86_64 only (BUILD-generation coverage doesn't need cross-platform).
