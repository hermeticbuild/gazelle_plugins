# import_extractor (binary toolchain)

Module extension that downloads the pre-built import-extractor Rust binary from
GitHub releases. Mirrors the pattern in
[`rules_formatjs/formatjs_cli/extensions.bzl`](https://github.com/formatjs/rules_formatjs/blob/main/formatjs_cli/extensions.bzl).

The `ts` Gazelle plugin's `data` dep points at `@import_extractor//:bin`, so
consumers don't pull `rules_rs` or build the Rust binary from source — they
just download the host-matching binary at fetch time.

## Default usage

```starlark
# MODULE.bazel
import_extractor = use_extension("@gazelle_ts//import_extractor:extensions.bzl", "import_extractor")
import_extractor.toolchain()
use_repo(import_extractor, "import_extractor")
```

That registers the latest known version (see `DEFAULT_VERSION` in
[`repositories.bzl`](repositories.bzl)) for both `darwin-arm64` and `linux-x64`.

## Pinning a version

```starlark
import_extractor.toolchain(version = "0.1.0")
```

Supported versions live in `IMPORT_EXTRACTOR_VERSIONS` (top of
[`repositories.bzl`](repositories.bzl)). Each new
`import_extractor_v*` GH release adds an entry; older versions stay so
downstream lockfiles don't break.

## Hosting on your own CDN

Pass `repositories` to override the URLs and SHA256s. Keys are
`<version>.<platform>`, values are `[url, sha256]`:

```starlark
import_extractor.toolchain(
    repositories = {
        "0.1.0.darwin-arm64": [
            "https://my.cdn/import_extractor-darwin-arm64",
            "240dcb1ff7dd0d6a488394365b78795f43bc8a4502bd062dfc29db953cd8cae2",
        ],
        "0.1.0.linux-x64": [
            "https://my.cdn/import_extractor-linux-x64",
            "97cd14ff983ee99395cd5f996a8917e3093d943c3f540acbee822662fe8e1531",
        ],
    },
)
```

This is the same shape rules_formatjs uses for `formatjs_repositories` — useful
when the public GH release URL is firewalled or you want a vendored mirror.

## Multi-tenant naming

Pass `name` to use multiple toolchains side-by-side (e.g. testing a new version
without disrupting the default):

```starlark
import_extractor.toolchain(name = "import_extractor_dev", version = "0.2.0-rc1")
use_repo(import_extractor, "import_extractor_dev")
```

`@import_extractor_dev//:bin` then resolves to the rc binary; the default
`@import_extractor//:bin` keeps pointing at the stable release.

## Why a single repo (no per-platform aggregator)

Gazelle always runs on the host, and the import-extractor isn't
cross-compiled. The repository_rule inspects `rctx.os` at fetch time and
downloads only the host-matching binary, so `@import_extractor//:bin` is a
plain `filegroup` — no `select()` / `config_setting` plumbing.

This also gives the Go side a stable runfiles path
(`import_extractor/import_extractor`) regardless of bzlmod canonicalization.
