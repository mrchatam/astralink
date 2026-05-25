package authority

import "testing"

func TestRegistrySingleSelect(t *testing.T) {
	r := NewRegistry(ModeSingle, []string{"10.0.0.1:53"})
	n, ok := r.Select()
	if !ok || n.Address != "10.0.0.1:53" {
		t.Fatalf("node=%+v ok=%v", n, ok)
	}
}

func TestRegistryMultiRoundRobin(t *testing.T) {
	r := NewRegistry(ModeMulti, []string{"a:53", "b:53"})
	first, _ := r.Select()
	second, _ := r.Select()
	if first.ID == second.ID {
		t.Fatal("expected rotation across healthy nodes")
	}
}
