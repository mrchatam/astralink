package authority

import "testing"

func TestBuildBootstrapMulti(t *testing.T) {
	info := BuildBootstrap(ModeMulti, []string{"a.example", "b.example"}, false)
	if info.Primary.Address == "" {
		t.Fatal("expected primary authority")
	}
	if len(info.Fallbacks) != 1 {
		t.Fatalf("fallbacks=%d", len(info.Fallbacks))
	}
}

func TestSelectClientAuthority(t *testing.T) {
	node, ok := SelectClientAuthority([]string{"ns1", "ns2"})
	if !ok || node.Address == "" {
		t.Fatal("expected authority node")
	}
}
