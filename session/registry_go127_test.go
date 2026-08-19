//go:build go1.27

package session

import (
	"reflect"
	"testing"
)

// The method is the function with the registry moved to the receiver, so the
// slot it declares has to land in the same registry the manager later freezes —
// including the duplicate checks, which are the registry's state rather than the
// call's.
func TestTheRegistryMethodDeclaresASlot(t *testing.T) {
	registry := NewRegistry()
	if err := registry.Register[payload]("account", Private, nil); err != nil {
		t.Fatalf("Register: %v", err)
	}
	slot, ok := registry.lookup(reflect.TypeFor[payload]())
	if !ok {
		t.Fatalf("the slot the method declared is not in the registry")
	}
	if slot.key != "account" || slot.placement != Private {
		t.Errorf("slot = %q %v, want account Private", slot.key, slot.placement)
	}
	if err := registry.Register[payload]("other", Private, nil); err == nil {
		t.Errorf("a second slot for the same type was accepted")
	}
	if err := Register[cart](registry, "account", Private, nil); err == nil {
		t.Errorf("the function accepted a key the method had already taken")
	}
}

// A nil registry is a caller error rather than a panic, and a method on a nil
// pointer has to report it the way the function does.
func TestTheRegistryMethodRefusesANilRegistry(t *testing.T) {
	var registry *Registry
	if err := registry.Register[payload]("account", Private, nil); err == nil {
		t.Errorf("a nil registry accepted a slot")
	}
}
