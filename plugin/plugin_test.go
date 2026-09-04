package plugin

import (
	"testing"
)

type fake struct {
	Base

	manifest Manifest
}

func (f *fake) Manifest() Manifest  { return f.manifest }
func (f *fake) Load(*Context) error { return nil }

func newFake(name string, depends ...string) *fake {
	return &fake{manifest: Manifest{Name: name, Version: "1.0.0", Depends: depends}}
}

func names(plugins []Plugin) []string {
	out := make([]string, 0, len(plugins))
	for _, p := range plugins {
		out = append(out, p.Manifest().Name)
	}
	return out
}

func TestOrderPutsDependenciesFirst(t *testing.T) {
	sorted, skipped, err := order([]Plugin{
		newFake("bans", "storage"),
		newFake("storage"),
		newFake("lobby", "bans"),
	})
	if err != nil {
		t.Fatalf("order failed: %v", err)
	}

	if len(skipped) != 0 {
		t.Fatalf("skipped %v plugins, expected none", len(skipped))
	}

	got := names(sorted)
	expected := []string{"storage", "bans", "lobby"}
	for i := range expected {
		if got[i] != expected[i] {
			t.Fatalf("order is %v, expected %v", got, expected)
		}
	}
}

func TestOrderSkipsAMissingDependency(t *testing.T) {
	sorted, skipped, err := order([]Plugin{newFake("bans", "storage"), newFake("lobby")})
	if err != nil {
		t.Fatalf("order failed: %v", err)
	}

	if got := names(sorted); len(got) != 1 || got[0] != "lobby" {
		t.Fatalf("loaded %v, expected only lobby", got)
	}

	if len(skipped) != 1 || skipped[0].Name != "bans" {
		t.Fatalf("skipped %v, expected bans", skipped)
	}
}

func TestOrderSkipsPluginsDependingOnASkippedOne(t *testing.T) {
	sorted, skipped, err := order([]Plugin{newFake("bans", "storage"), newFake("kits", "bans")})
	if err != nil {
		t.Fatalf("order failed: %v", err)
	}

	if len(sorted) != 0 {
		t.Fatalf("loaded %v, expected none", names(sorted))
	}

	if len(skipped) != 2 {
		t.Fatalf("skipped %v plugins, expected 2", len(skipped))
	}
}

func TestOrderRejectsACycle(t *testing.T) {
	if _, _, err := order([]Plugin{newFake("a", "b"), newFake("b", "a")}); err == nil {
		t.Fatal("expected a circular dependency error")
	}
}

func TestOrderRejectsDuplicateNames(t *testing.T) {
	if _, _, err := order([]Plugin{newFake("bans"), newFake("bans")}); err == nil {
		t.Fatal("expected a duplicate name error")
	}
}
