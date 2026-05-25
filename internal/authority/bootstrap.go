package authority

import (
	"strings"
)

// BootstrapInfo describes authority endpoints for client session bootstrap.
type BootstrapInfo struct {
	Mode       Mode
	Primary    Node
	Fallbacks  []Node
	AnycastHint bool
}

// BuildBootstrap assembles bootstrap data from mode and peer list.
func BuildBootstrap(mode Mode, peers []string, anycast bool) BootstrapInfo {
	reg := NewRegistry(mode, peers)
	primary, _ := reg.Select()
	fallbacks := make([]Node, 0)
	for _, n := range reg.Nodes() {
		if n.ID != primary.ID {
			fallbacks = append(fallbacks, n)
		}
	}
	return BootstrapInfo{
		Mode:        mode,
		Primary:     primary,
		Fallbacks:   fallbacks,
		AnycastHint: anycast,
	}
}

// ClientAuthorityEndpoints parses configured authority endpoints.
func ClientAuthorityEndpoints(raw []string) []string {
	out := make([]string, 0, len(raw))
	for _, line := range raw {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

// SelectClientAuthority picks primary authority for client bootstrap.
func SelectClientAuthority(endpoints []string) (Node, bool) {
	if len(endpoints) == 0 {
		return Node{}, false
	}
	reg := NewRegistry(ModeMulti, endpoints)
	return reg.Select()
}
