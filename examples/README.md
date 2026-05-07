# examples

Each subdirectory is a self-contained Bazel workspace exercising the `ts` plugin against a different scenario. They escalate in complexity:

| Example | What it shows |
|---|---|
| [`basic/`](basic) | One TS package, third-party npm deps (`lodash-es`, `react-intl`), a `.tsx` file, a smoke test. **No internal cross-package references.** Smallest possible useful setup. |
| [`bundler-config/`](bundler-config) | `ts_bundler_config_pattern` peeling vite, vitest, and tailwind configs out of the library compilation unit. Demonstrates `map_kind` to a concrete macro and verifies the lib→config import boundary fails at typecheck. |
| [`composite/`](composite) | Multiple packages with cross-package imports via `package.json` `imports` (`#packages/*`). TS project references via `composite = True`. |
| [`graphql/`](graphql) | Real `@graphql-codegen/cli` running as a Bazel build step against `.graphql` files; the output is compiled by `ts_project`, wrapped as `npm_package`, linked at `//:node_modules/@myrepo_generated/queries`, and consumed by a composite app via `# gazelle:resolve_regexp`. |
| [`advanced/`](advanced) | Everything in `composite` plus a Bazel-generated synthetic npm package (genrule + `npm_package` + `npm_link_package`) wired through the `# gazelle:resolve_regexp` directive. |

Each workspace points its `MODULE.bazel` at the parent `gazelle_ts` repo via `local_path_override`, so changes to the plugin's Go source apply on the next `bazel run //:gazelle` without any release dance.

CI runs `bazel test //...` and a `gazelle update -mode=diff` idempotency check in every example on linux-x86_64 + macos-arm64.
