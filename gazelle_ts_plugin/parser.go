package ts

// ImportStatement is a single import found in a TypeScript file. We track the
// source file so error messages can point at the exact file that introduced a
// particular dependency.
type ImportStatement struct {
	ImportPath string // module specifier (e.g., "react", "#packages/foo/x.js")
	SourceFile string // file containing the import (e.g., "packages/foo/src/index.ts")
}

// extractImportsBatch sends a batch of file paths to the Rust subprocess
// (one round-trip total) and returns the parsed imports keyed by file path.
// Per-file rather than per-call batching keeps Rust's rayon parallelism alive.
func (l *tsLang) extractImportsBatch(filePaths []string) (map[string][]ImportStatement, error) {
	if l.parser == nil {
		return nil, nil
	}

	result, err := l.parser.ExtractImports(filePaths)
	if err != nil {
		return nil, err
	}

	imports := make(map[string][]ImportStatement, len(result))
	for file, paths := range result {
		stmts := make([]ImportStatement, 0, len(paths))
		for _, p := range paths {
			stmts = append(stmts, ImportStatement{
				ImportPath: p,
				SourceFile: file,
			})
		}
		imports[file] = stmts
	}
	return imports, nil
}
