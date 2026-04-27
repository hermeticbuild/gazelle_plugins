# examples/custom-extractor

Same as [`examples/basic`](../basic), but the `import_extractor` module
extension is wired with the `repositories = {...}` override instead of the
default URL/SHA256 map shipped in
[`//import_extractor:repositories.bzl`](../../import_extractor/repositories.bzl).

## When you'd use this

- The public GitHub release is firewalled or rate-limited; mirror the binary
  on your own CDN and point the override there.
- You're shipping a fork or pre-release that doesn't yet have an entry in
  `IMPORT_EXTRACTOR_VERSIONS`.
- You want to pin a specific integrity hash that your security team has
  audited, separate from whatever's in the upstream map.

## Wiring

See [`MODULE.bazel`](MODULE.bazel). The relevant block:

```starlark
import_extractor = use_extension("@gazelle_ts//import_extractor:extensions.bzl", "import_extractor")
import_extractor.toolchain(
    version = "0.1.1",
    repositories = {
        "0.1.1.darwin-arm64": ["https://my.cdn/import_extractor-darwin-arm64", "<sha256>"],
        "0.1.1.linux-x64":    ["https://my.cdn/import_extractor-linux-x64",    "<sha256>"],
    },
)
use_repo(import_extractor, "import_extractor")
```

Keys are `"<version>.<platform>"`; values are `[url, sha256_hex]`. The
extension picks the host-matching entry at fetch time. If a `(version, platform)`
key isn't in the override, it falls through to the built-in map.

The URLs in this example happen to point at the public GH release so the
example actually works end-to-end — swap the host for your own CDN to verify
the override path in your environment.

## Try it

```bash
bazel run //:gazelle     # downloads via the override URL on first run
bazel build //...
bazel test //...
```
