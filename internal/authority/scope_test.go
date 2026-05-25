package authority

import "testing"

func TestRegistryMarkHealthDoesNotChangeBootstrapContract(t *testing.T) {
	reg := NewRegistry(ModeMulti, []string{"a.example", "b.example"})
	reg.MarkHealth("b.example", false)
	n1, ok := reg.Select()
	if !ok {
		t.Fatal("expected selection")
	}
	n2, ok := reg.Select()
	if !ok {
		t.Fatal("expected second selection")
	}
	if n1.Address == "" || n2.Address == "" {
		t.Fatal("round-robin must return configured peers only")
	}
}
