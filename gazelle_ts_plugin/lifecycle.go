package ts

import (
	"context"
	"log"

	"github.com/bazelbuild/bazel-gazelle/language"
)

// lifeCycleManager owns the parser subprocess. Embedding
// language.BaseLifecycleManager keeps us forward-compatible with new hooks.
type lifeCycleManager struct {
	language.BaseLifecycleManager
	parser *ImportExtractor
}

// Before is called once at plugin startup; we spawn the Rust subprocess here.
func (l *lifeCycleManager) Before(ctx context.Context) {
	p, err := newImportExtractor()
	if err != nil {
		log.Printf("ts: import-extractor unavailable: %v", err)
		return
	}
	l.parser = p
}

// DoneGeneratingRules is called after every directory has been scanned; we
// shut down the subprocess.
func (l *lifeCycleManager) DoneGeneratingRules() {
	if l.parser != nil {
		if err := l.parser.Close(); err != nil {
			log.Printf("ts: import-extractor shutdown error: %v", err)
		}
		l.parser = nil
	}
}

// AfterResolvingDeps is a no-op.
func (l *lifeCycleManager) AfterResolvingDeps(ctx context.Context) {}
