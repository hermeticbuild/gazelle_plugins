"""Per-platform binary URLs + integrity hashes for the import-extractor.

Two layers:

  IMPORT_EXTRACTOR_VERSIONS  — built-in matrix of `version → {platform → {url, sha256}}`.
                               Updated when a new GH release is cut.
  _import_extractor_repo     — the repository_rule that downloads the
                               host-matching binary and exposes it as `:bin`.

Single-repo design (no per-platform aggregator) because gazelle always runs on
the host and we don't cross-compile. Keeping the binary in one apparent repo
gives the Go side a stable runfiles path (`import_extractor/import_extractor`)
regardless of bzlmod canonicalization.
"""

# Updated whenever a new `import_extractor_v*` release is cut. Each platform
# entry is `{url, sha256}`. SHA256s are over the raw binary file.
IMPORT_EXTRACTOR_VERSIONS = {
    "0.1.1": {
        "darwin-arm64": {
            "url": "https://github.com/hermeticbuild/gazelle_ts/releases/download/import_extractor_v0.1.1/import_extractor-darwin-arm64",
            "sha256": "c22df4be91126f2739bf1f1cba55bd922eaa6ded78d8a5d40aa8728cbd5c9fb7",
        },
        "linux-x64": {
            "url": "https://github.com/hermeticbuild/gazelle_ts/releases/download/import_extractor_v0.1.1/import_extractor-linux-x64",
            "sha256": "74954c35702d251baf8e8f317390b33a0245f53e4c973f4347022198927ea4af",
        },
    },
    "0.1.0": {
        "darwin-arm64": {
            "url": "https://github.com/hermeticbuild/gazelle_ts/releases/download/import_extractor_v0.1.0/import_extractor-darwin-arm64",
            "sha256": "240dcb1ff7dd0d6a488394365b78795f43bc8a4502bd062dfc29db953cd8cae2",
        },
        "linux-x64": {
            "url": "https://github.com/hermeticbuild/gazelle_ts/releases/download/import_extractor_v0.1.0/import_extractor-linux-x64",
            "sha256": "97cd14ff983ee99395cd5f996a8917e3093d943c3f540acbee822662fe8e1531",
        },
    },
}

DEFAULT_VERSION = "0.1.1"

def _host_platform_key(rctx):
    """Map repository_ctx host info → keys used in IMPORT_EXTRACTOR_VERSIONS."""
    name, arch = rctx.os.name, rctx.os.arch
    if name == "mac os x" and arch == "aarch64":
        return "darwin-arm64"
    if name == "linux" and arch in ("x86_64", "amd64"):
        return "linux-x64"
    fail(
        "import_extractor: unsupported host platform os=%r arch=%r. " % (name, arch) +
        "Override via `import_extractor.toolchain(repositories = {...})` to ship " +
        "your own binary.",
    )

def _import_extractor_repo_impl(rctx):
    platform_key = _host_platform_key(rctx)
    key = "%s.%s" % (rctx.attr.version, platform_key)

    overrides = rctx.attr.repositories
    if key in overrides:
        url, sha256 = overrides[key]
    else:
        if rctx.attr.version not in IMPORT_EXTRACTOR_VERSIONS:
            fail((
                "import_extractor: version %r is not in the built-in version map " +
                "(%s). Pass `repositories = {...}` to point at a custom URL."
            ) % (rctx.attr.version, ", ".join(sorted(IMPORT_EXTRACTOR_VERSIONS.keys()))))
        entry = IMPORT_EXTRACTOR_VERSIONS[rctx.attr.version].get(platform_key)
        if entry == None:
            fail("import_extractor: no built-in entry for %s on %s" % (rctx.attr.version, platform_key))
        url, sha256 = entry["url"], entry["sha256"]

    rctx.download(
        url = url,
        sha256 = sha256,
        output = "import_extractor",
        executable = True,
    )
    rctx.file("BUILD.bazel", """\
filegroup(
    name = "bin",
    srcs = ["import_extractor"],
    visibility = ["//visibility:public"],
)
""")

import_extractor_repo = repository_rule(
    implementation = _import_extractor_repo_impl,
    attrs = {
        "version": attr.string(mandatory = True),
        "repositories": attr.string_list_dict(
            doc = "Override the built-in URL/SHA256 map. Keys are " +
                  "'<version>.<platform>'; values are [url, sha256].",
        ),
    },
    doc = "Downloads the host-matching import-extractor binary and exposes it as :bin.",
)
