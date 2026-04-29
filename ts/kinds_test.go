package ts

import "testing"

func TestEnsureResolveAttr_AddsToExistingKind(t *testing.T) {
	l := &tsLang{kindInfos: defaultKindInfos()}
	l.ensureResolveAttr(defaultLibraryKind, "vite_config_deps")

	info := l.kindInfos[defaultLibraryKind]
	if !info.ResolveAttrs["vite_config_deps"] {
		t.Errorf("vite_config_deps not registered: %v", info.ResolveAttrs)
	}
	// Stock attrs untouched.
	if !info.ResolveAttrs["deps"] {
		t.Errorf("stock deps attr lost: %v", info.ResolveAttrs)
	}
}

func TestEnsureResolveAttr_SynthesizesUnknownKind(t *testing.T) {
	l := &tsLang{kindInfos: defaultKindInfos()}
	l.ensureResolveAttr("custom_lib_kind", "my_attr")

	info, ok := l.kindInfos["custom_lib_kind"]
	if !ok {
		t.Fatalf("custom_lib_kind not registered")
	}
	if !info.ResolveAttrs["my_attr"] {
		t.Errorf("my_attr not registered: %v", info.ResolveAttrs)
	}
	if !info.NonEmptyAttrs["name"] {
		t.Errorf("expected synthesized NonEmptyAttrs to include name: %v", info.NonEmptyAttrs)
	}
}

func TestEnsureResolveAttr_Idempotent(t *testing.T) {
	l := &tsLang{kindInfos: defaultKindInfos()}
	l.ensureResolveAttr(defaultLibraryKind, "vite_config_deps")
	l.ensureResolveAttr(defaultLibraryKind, "vite_config_deps")
	if got := len(l.kindInfos[defaultLibraryKind].ResolveAttrs); got != 2 {
		t.Errorf("ResolveAttrs size = %d after duplicate registration, want 2 (deps + vite_config_deps)", got)
	}
}

func TestEnsureResolveAttr_IgnoresEmptyArgs(t *testing.T) {
	l := &tsLang{kindInfos: defaultKindInfos()}
	before := len(l.kindInfos)
	l.ensureResolveAttr("", "x")
	l.ensureResolveAttr("y", "")
	if len(l.kindInfos) != before {
		t.Errorf("empty args should be ignored; kindInfos size changed from %d to %d", before, len(l.kindInfos))
	}
}

// Two language instances must not share state via aliased KindInfo maps —
// otherwise a directive in one tree could leak ResolveAttrs into another
// (e.g. when gazelle is invoked twice in the same process by tests).
func TestDefaultKindInfos_IndependentInstances(t *testing.T) {
	a := defaultKindInfos()
	b := defaultKindInfos()
	a[defaultLibraryKind].ResolveAttrs["leaked"] = true
	if b[defaultLibraryKind].ResolveAttrs["leaked"] {
		t.Errorf("ResolveAttrs map aliased across defaultKindInfos() calls")
	}
}
