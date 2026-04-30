# gazelle_ts

Bazel build setup, a Gazelle TypeScript language extension, and the Rust import-extractor that powers it (linked in via cgo).

Built on **Bazel 8.5+ (bzlmod)** with [`rules_rs`](https://github.com/dzbarsky/rules_rs) for the Rust side and `aspect_rules_ts` / `aspect_rules_js` for the TypeScript examples. Tested in CI against Bazel 8.5.1 and 9.0.0.

## Layout

```
crates/
└── import_extractor/         # Rust staticlib: TS import extraction (oxc).
                              # Linked into the gazelle plugin via cgo.
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
- **`crates/import_extractor`** — Rust staticlib that parses TypeScript imports via `oxc`. Exposes a small C ABI (`ie_dispatch` / `ie_free`); the gazelle plugin links it via cgo and dispatches in-process. See [`crates/import_extractor/README.md`](crates/import_extractor/README.md).
- **`examples/`** — four escalating example workspaces, each with its own `MODULE.bazel`, `pnpm-lock.yaml`, and `tsconfig.json`. They `local_path_override` the parent module so plugin changes apply on the next `bazel run //:gazelle`. See [`examples/README.md`](examples/README.md).

## Consumer setup

Add the module and compose your own `gazelle_binary`:

```python
# MODULE.bazel
bazel_dep(name = "gazelle", version = "0.50.0")
bazel_dep(name = "gazelle_ts", version = "<latest>")

# Required so the consumer .bazelrc below can reference @llvm directly —
# bzlmod doesn't transitively expose deps' repos.
bazel_dep(name = "llvm", version = "0.7.6")
```

> [!NOTE]
> `gazelle_ts` registers a hermetic `@llvm` cc toolchain (so the rules_rs Rust toolchain doesn't trip Bazel's Xcode autodetect on macOS). To use it from a consumer workspace you have to mirror the flags below in your own `.bazelrc` — Bazel only reads the consumer's rc, not a dep's:
>
> ```
> common --enable_platform_specific_config
>
> # Linux/Windows: pin host_platform so rules_rs's Rust toolchains match the
> # gnu.2.28 libc / msvc constraints they tag via target_compatible_with.
> common:linux --host_platform=@gazelle_ts//platforms:local_gnu
> common:windows --host_platform=@gazelle_ts//platforms:local_windows_msvc
>
> # Suppress Bazel's autodetected cc toolchain so @llvm wins resolution
> # cleanly. NO_APPLE specifically avoids the XcodeLocalEnvProvider
> # duplicate-SDKROOT crash on macOS.
> common --repo_env=BAZEL_DO_NOT_DETECT_CPP_TOOLCHAIN=1
> common --repo_env=BAZEL_NO_APPLE_CPP_TOOLCHAIN=1
>
> # rust stdlib's link spec hardcodes -lgcc_s; @llvm's clang doesn't
> # ship it, so we inject an empty stub.
> common --@llvm//config:experimental_stub_libgcc_s=True
>
> # rules_go cgo external link via clang+lld can't produce PIE. Drop
> # when Go 1.27 (Aug 2026) lands PIE-compatible objects.
> build:linux --linkopt=-no-pie
> ```
>
> See [`examples/basic/.bazelrc`](examples/basic/.bazelrc) for a working setup.

```python
# BUILD.bazel
load("@gazelle//:def.bzl", "gazelle", "gazelle_binary")

# gazelle:ts_npm_link_pattern //:node_modules/{pkg}
# gazelle:ts_tsconfig //:tsconfig

gazelle_binary(
    name = "gazelle_bin",
    languages = ["@gazelle_ts//ts"],
)

gazelle(
    name = "gazelle",
    gazelle = ":gazelle_bin",
)
```

Then `bazel run //:gazelle` to generate `ts_project` / `js_test` rules. Swap to your own macros via `# gazelle:map_kind`. See [`ts/README.md`](ts/README.md) for the directive reference and [`examples/`](examples/) for end-to-end workspaces.

## Build

```
bazel test //...
```

Tests in `crates/` and `ts/` run on linux-x86_64 + macos-arm64 in CI; the example workspaces run on linux-x86_64 only (BUILD-generation coverage doesn't need cross-platform).
