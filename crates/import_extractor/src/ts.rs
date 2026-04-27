// extract_imports.rs -- oxc-based TypeScript import extractor.
//
// Parses TypeScript/TSX files with oxc and walks the AST to collect all import paths.
// This is the core parsing logic used by the gazelle TS plugin to determine
// dependencies between TypeScript packages.
//
// Handles all TypeScript import forms:
//   - import declarations:    import { x } from 'module', import 'module'
//   - export-from:            export { x } from 'module'
//   - export-all:             export * from 'module'
//   - dynamic import:         import('module')  (oxc models this as ImportExpression)
//   - type-only imports:      import type { X } from 'module' (still extracted -- needed for type checking)
//
// The extracted paths are raw module specifiers (e.g., "react", "myorg/frontend/common").
// Resolution to Bazel labels happens on the Go side in resolve.go.

use oxc::ast_visit::{Visit, walk};
use oxc_allocator::Allocator;
use oxc_ast::ast::*;
use oxc_parser::Parser;
use oxc_span::SourceType;
use std::collections::HashSet;

/// Extract all import paths from a TypeScript/TSX file on disk.
pub fn extract_imports_from_file(path: &str) -> Result<Vec<String>, String> {
    let source_text =
        std::fs::read_to_string(path).map_err(|e| format!("Failed to read {path}: {e}"))?;
    Ok(extract_imports(path, &source_text))
}

/// Extract all import paths from TypeScript/TSX source code.
///
/// For malformed files, oxc performs error recovery and produces a partial AST.
/// We extract imports from whatever the parser could recover, which is the right
/// behavior for gazelle: partially-edited files during development still get
/// their valid imports resolved. Since gazelle runs as a pre-commit hook, the
/// file will typically be fixed before the next run.
pub fn extract_imports(path: &str, source_text: &str) -> Vec<String> {
    let allocator = Allocator::default();
    // SourceType::from_path sets jsx=true for .tsx, jsx=false for .ts/.mts/.cts.
    // Do NOT override with with_jsx(true) — it causes oxc to misparse TypeScript
    // generics (e.g. Promise<Foo | undefined>) as JSX opening tags in .ts files,
    // silently mangling the AST and losing imports from function bodies.
    let source_type = SourceType::from_path(path).unwrap_or_default();

    let ret = Parser::new(&allocator, source_text, source_type).parse();

    let mut visitor = ImportVisitor::new();
    visitor.visit_program(&ret.program);
    visitor.into_imports()
}

/// AST visitor that collects import paths from TypeScript source code.
struct ImportVisitor {
    imports: Vec<String>,
    seen: HashSet<String>,
}

impl ImportVisitor {
    fn new() -> Self {
        Self {
            imports: Vec::new(),
            seen: HashSet::new(),
        }
    }

    fn add(&mut self, path: &str) {
        if !path.is_empty() && self.seen.insert(path.to_string()) {
            self.imports.push(path.to_string());
        }
    }

    fn into_imports(self) -> Vec<String> {
        self.imports
    }
}

impl<'a> Visit<'a> for ImportVisitor {
    // import ... from 'module' | import 'module'
    fn visit_import_declaration(&mut self, decl: &ImportDeclaration<'a>) {
        self.add(decl.source.value.as_str());
        walk::walk_import_declaration(self, decl);
    }

    // export { x } from 'module'
    fn visit_export_named_declaration(&mut self, decl: &ExportNamedDeclaration<'a>) {
        if let Some(ref source) = decl.source {
            self.add(source.value.as_str());
        }
        walk::walk_export_named_declaration(self, decl);
    }

    // export * from 'module'
    fn visit_export_all_declaration(&mut self, decl: &ExportAllDeclaration<'a>) {
        self.add(decl.source.value.as_str());
        walk::walk_export_all_declaration(self, decl);
    }

    // import('module') -- oxc models dynamic imports as ImportExpression
    fn visit_import_expression(&mut self, expr: &ImportExpression<'a>) {
        if let Expression::StringLiteral(lit) = &expr.source {
            self.add(lit.value.as_str());
        }
        walk::walk_import_expression(self, expr);
    }

    // import('module').Type -- oxc models inline type imports as TSImportType
    fn visit_ts_import_type(&mut self, it: &TSImportType<'a>) {
        self.add(it.source.value.as_str());
        walk::walk_ts_import_type(self, it);
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn empty_file() {
        assert_eq!(extract_imports("test.ts", ""), Vec::<String>::new());
    }

    #[test]
    fn static_imports() {
        let imports = extract_imports(
            "test.ts",
            r#"
            import foo from 'foo';
            import { bar } from 'bar';
            import * as baz from 'baz';
        "#,
        );
        assert_eq!(imports, vec!["foo", "bar", "baz"]);
    }

    #[test]
    fn side_effect_import() {
        let imports = extract_imports("test.ts", "import 'polyfill';");
        assert_eq!(imports, vec!["polyfill"]);
    }

    #[test]
    fn type_imports() {
        let imports = extract_imports(
            "test.ts",
            r#"
            import type { Foo } from 'foo';
            import { type Bar } from 'bar';
        "#,
        );
        assert_eq!(imports, vec!["foo", "bar"]);
    }

    #[test]
    fn export_from() {
        let imports = extract_imports(
            "test.ts",
            r#"
            export { x } from 'foo';
            export * from 'bar';
            export type { Baz } from 'baz';
        "#,
        );
        assert_eq!(imports, vec!["foo", "bar", "baz"]);
    }

    #[test]
    fn dynamic_import() {
        let imports = extract_imports("test.ts", "const m = await import('lazy');");
        assert_eq!(imports, vec!["lazy"]);
    }

    #[test]
    fn tsx_file() {
        let imports = extract_imports(
            "test.tsx",
            r#"
            import React from 'react';
            export function App() { return <div />; }
        "#,
        );
        assert_eq!(imports, vec!["react"]);
    }

    #[test]
    fn deduplicates() {
        let imports = extract_imports(
            "test.ts",
            r#"
            import { a } from 'foo';
            import { b } from 'foo';
        "#,
        );
        assert_eq!(imports, vec!["foo"]);
    }

    #[test]
    fn workspace_imports() {
        let imports = extract_imports(
            "test.ts",
            r#"
            import { x } from 'myorg/frontend/common';
            import type { User } from 'myorg/frontend/types';
        "#,
        );
        assert_eq!(
            imports,
            vec!["myorg/frontend/common", "myorg/frontend/types"]
        );
    }

    #[test]
    fn mixed_imports() {
        let imports = extract_imports(
            "test.ts",
            r#"
            import React from 'react';
            import { helper } from 'myorg/frontend/utils';
            import type { Props } from 'myorg/frontend/types';
            export * from './utils.js';
            const lazy = await import('lazy-module');
        "#,
        );
        assert_eq!(
            imports,
            vec![
                "react",
                "myorg/frontend/utils",
                "myorg/frontend/types",
                "./utils.js",
                "lazy-module",
            ]
        );
    }

    #[test]
    fn ignores_comments() {
        let imports = extract_imports(
            "test.ts",
            r#"
            // import React from 'react';
            /* import { useState } from 'react'; */
            import { useEffect } from 'react-dom';
        "#,
        );
        assert_eq!(imports, vec!["react-dom"]);
    }

    #[test]
    fn ignores_string_literals() {
        let imports = extract_imports(
            "test.ts",
            r#"
            const code = "import React from 'react';";
            import { useState } from 'react-dom';
        "#,
        );
        assert_eq!(imports, vec!["react-dom"]);
    }

    #[test]
    fn side_effect_css() {
        let imports = extract_imports(
            "test.ts",
            r#"
            import './styles.css';
            import '../reset.css';
            import 'tailwindcss/base';
        "#,
        );
        assert_eq!(
            imports,
            vec!["./styles.css", "../reset.css", "tailwindcss/base"]
        );
    }

    #[test]
    fn side_effect_polyfills() {
        let imports = extract_imports(
            "test.ts",
            r#"
            import 'core-js/stable';
            import 'regenerator-runtime/runtime';
            import '@formatjs/intl-pluralrules/polyfill';
            import '@formatjs/intl-pluralrules/locale-data/en';
        "#,
        );
        assert_eq!(
            imports,
            vec![
                "core-js/stable",
                "regenerator-runtime/runtime",
                "@formatjs/intl-pluralrules/polyfill",
                "@formatjs/intl-pluralrules/locale-data/en",
            ]
        );
    }

    #[test]
    fn side_effect_mixed_with_regular() {
        let imports = extract_imports(
            "test.ts",
            r#"
            import 'reflect-metadata';
            import { Injectable } from 'tsyringe';
            import './instrument';
            import { App } from './App';
        "#,
        );
        assert_eq!(
            imports,
            vec!["reflect-metadata", "tsyringe", "./instrument", "./App"]
        );
    }

    #[test]
    fn side_effect_not_extracted_from_comments() {
        let imports = extract_imports(
            "test.ts",
            r#"
            // import 'old-polyfill';
            /* import 'removed-shim'; */
            import 'actual-polyfill';
        "#,
        );
        assert_eq!(imports, vec!["actual-polyfill"]);
    }

    #[test]
    fn side_effect_not_extracted_from_strings() {
        let imports = extract_imports(
            "test.ts",
            r#"
            const code = "import 'fake-polyfill';";
            const tmpl = `import 'template-polyfill';`;
            import 'real-polyfill';
        "#,
        );
        assert_eq!(imports, vec!["real-polyfill"]);
    }

    #[test]
    fn side_effect_deduplicates() {
        let imports = extract_imports(
            "test.ts",
            r#"
            import 'myorg/frontend/styles';
            import 'myorg/frontend/styles';
        "#,
        );
        assert_eq!(imports, vec!["myorg/frontend/styles"]);
    }

    #[test]
    fn scoped_packages() {
        let imports = extract_imports(
            "test.ts",
            r#"
            import { useQuery } from '@tanstack/react-query';
            import { Button } from '@mui/material';
            import '@sentry/nextjs';
        "#,
        );
        assert_eq!(
            imports,
            vec!["@tanstack/react-query", "@mui/material", "@sentry/nextjs"]
        );
    }

    #[test]
    fn subpath_imports() {
        let imports = extract_imports(
            "test.ts",
            r#"
            import debounce from 'lodash/debounce';
            import { DevTools } from '@tanstack/react-query/devtools';
        "#,
        );
        assert_eq!(
            imports,
            vec!["lodash/debounce", "@tanstack/react-query/devtools"]
        );
    }

    #[test]
    fn node_builtins() {
        let imports = extract_imports(
            "test.ts",
            r#"
            import path from 'node:path';
            import { readFileSync } from 'fs';
            import { createServer } from 'node:http';
        "#,
        );
        assert_eq!(imports, vec!["node:path", "fs", "node:http"]);
    }

    #[test]
    fn multiline_imports() {
        let imports = extract_imports(
            "test.ts",
            r#"
            import {
                useState,
                useEffect,
                useMemo,
            } from 'react';
            import type {
                User,
                Post,
            } from 'myorg/frontend/types';
        "#,
        );
        assert_eq!(imports, vec!["react", "myorg/frontend/types"]);
    }

    #[test]
    fn require_not_extracted() {
        let imports = extract_imports(
            "test.ts",
            r#"
            const React = require('react');
            require('side-effect');
            import { useState } from 'react-dom';
        "#,
        );
        assert_eq!(imports, vec!["react-dom"]);
    }

    #[test]
    fn hashbang_imports() {
        let imports = extract_imports(
            "test.ts",
            r#"
            import { x } from '#myorg/frontend/common';
            import '#myorg/frontend/styles';
        "#,
        );
        assert_eq!(
            imports,
            vec!["#myorg/frontend/common", "#myorg/frontend/styles"]
        );
    }

    #[test]
    fn malformed_file_does_not_panic() {
        // oxc does error recovery — the key contract is no panics on malformed input
        let imports = extract_imports(
            "test.ts",
            r#"
            import { foo } from 'valid-before';
            const x = {{{;  // syntax error
            import { bar } from 'valid-after';
        "#,
        );
        // oxc may recover some or all imports depending on how the error recovery works.
        // The important thing is that it doesn't panic.
        let _ = imports;
    }

    #[test]
    fn completely_malformed_file() {
        // Should not panic, returns whatever oxc can recover (possibly empty)
        let imports = extract_imports("test.ts", "}{}{}{}{{{{}}}");
        // Just verify it doesn't panic — result may be empty
        let _ = imports;
    }

    #[test]
    fn empty_import_path() {
        // import from '' — oxc parses it, we filter empty strings
        let imports = extract_imports("test.ts", "import '' ;");
        assert!(imports.is_empty());
    }

    #[test]
    fn dynamic_import_in_function_body() {
        let imports = extract_imports(
            "test.ts",
            "import { foo } from 'static';\nasync function f() { await import('dynamic'); }",
        );
        assert!(
            imports.contains(&"dynamic".to_string()),
            "Missing dynamic import inside function body. Got: {:?}",
            imports
        );
    }

    #[test]
    fn inline_import_type_access() {
        // Pattern: import('postcss').Root — used for inline type annotations
        // e.g. in vite configs: Once(root: import('postcss').Root) { ... }
        let imports = extract_imports(
            "test.ts",
            r#"
            const plugin = {
                postcssPlugin: 'my-plugin',
                Once(root: import('postcss').Root) {
                    root.walkRules((rule: import('postcss').Rule) => {});
                },
            };
        "#,
        );
        assert!(
            imports.contains(&"postcss".to_string()),
            "inline import('postcss').Type not extracted. Got: {:?}",
            imports
        );
    }

    #[test]
    fn dynamic_import_with_generic_return_type() {
        let imports = extract_imports(
            "test.ts",
            "import { foo } from 'static';\nexport async function f(): Promise<string | undefined> { return import('dynamic'); }",
        );
        assert!(imports.contains(&"static".to_string()));
        assert!(imports.contains(&"dynamic".to_string()));
    }
}
