# gazelle_plugins

Bazel build setup and shared tooling for gazelle plugins, including the Rust
[`import-extractor`](crates/import-extractor) subprocess used by the gazelle TS
and Python plugins.

Built with [`rules_rs`](https://github.com/dzbarsky/rules_rs) on Bazel 9 (bzlmod).

## Layout

```
crates/
├── import-extractor/        # Rust subprocess: TS + Python import extraction
└── import-extractor-proto/  # Wire-protocol protobuf schema
```

## Build

```
bazel test //...
```

See [`crates/import-extractor/README.md`](crates/import-extractor/README.md) for
the wire protocol and details.
