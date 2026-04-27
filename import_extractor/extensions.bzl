"""Module extension that downloads the import-extractor Rust binary.

Default usage (in a consumer's MODULE.bazel):

    import_extractor = use_extension(
        "@gazelle_plugins//import_extractor:extensions.bzl",
        "import_extractor",
    )
    import_extractor.toolchain()
    use_repo(import_extractor, "import_extractor")

`@import_extractor//:bin` then resolves to a host-matching binary. Pin to a
specific version with `import_extractor.toolchain(version = "0.1.0")`.

Override URLs/hashes (e.g. to host the binary on your own CDN) with
`repositories`:

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

Modeled on rules_formatjs's formatjs_cli extension. Single-repo (no per-platform
aggregator) because gazelle always runs on the host — see repositories.bzl.
"""

load(":repositories.bzl", "DEFAULT_VERSION", "import_extractor_repo")

def _import_extractor_impl(module_ctx):
    # Collect tags across modules. The root module's tag wins over a
    # transitive (gazelle_plugins') one when names collide — so a consumer
    # who pins `version` overrides the default we ship.
    selected = {}
    for mod in module_ctx.modules:
        for tag in mod.tags.toolchain:
            name = tag.name or "import_extractor"
            if mod.is_root or name not in selected:
                selected[name] = tag

    for name, tag in selected.items():
        version = tag.version or DEFAULT_VERSION
        import_extractor_repo(
            name = name,
            version = version,
            repositories = tag.repositories,
        )

_toolchain = tag_class(
    attrs = {
        "name": attr.string(
            doc = "Apparent repo name. Defaults to `import_extractor`.",
        ),
        "version": attr.string(
            doc = "Released import-extractor version. Defaults to %s." % DEFAULT_VERSION,
        ),
        "repositories": attr.string_list_dict(
            doc = (
                "Override the built-in URL/SHA256 map. Keys are " +
                "'<version>.<platform>' (e.g. '0.1.0.darwin-arm64'); values are " +
                "[url, sha256]. Empty by default — uses IMPORT_EXTRACTOR_VERSIONS."
            ),
        ),
    },
)

import_extractor = module_extension(
    implementation = _import_extractor_impl,
    tag_classes = {"toolchain": _toolchain},
)
