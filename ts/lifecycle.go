package ts

import (
	"context"

	"github.com/bazelbuild/bazel-gazelle/language"
)

// lifeCycleManager owns the import-extractor handle. The Rust code is linked
// into this binary via cgo (see import_extractor.go), so there's no
// subprocess to spawn or shut down — Before/DoneGeneratingRules just create
// and drop a handle.
type lifeCycleManager struct {
	language.BaseLifecycleManager
	parser *ImportExtractor
}

func (l *lifeCycleManager) Before(ctx context.Context) {
	l.parser, _ = newImportExtractor()
}

func (l *lifeCycleManager) DoneGeneratingRules() {
	l.parser = nil
}

func (l *lifeCycleManager) AfterResolvingDeps(ctx context.Context) {}
