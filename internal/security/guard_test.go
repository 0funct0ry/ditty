package security

import (
	"fmt"
	"testing"
)

func TestBindGuard(t *testing.T) {
	tests := []struct {
		name             string
		addr             string
		isUnixSocket     bool
		grantsConfigured bool
		insecureNoAuth   bool
		wantErr          bool
	}{
		{name: "loopback, no grant", addr: "127.0.0.1:7654", wantErr: false},
		{name: "loopback ipv6, no grant", addr: "[::1]:7654", wantErr: false},
		{name: "localhost, no grant", addr: "localhost:7654", wantErr: false},
		{name: "LAN, no grant", addr: "192.168.1.24:7654", wantErr: true},
		{name: "0.0.0.0, no grant", addr: "0.0.0.0:7654", wantErr: true},
		{name: "unix socket, no grant", addr: "/tmp/ditty.sock", isUnixSocket: true, wantErr: false},
		{name: "LAN, token grant", addr: "192.168.1.24:7654", grantsConfigured: true, wantErr: false},
		{name: "0.0.0.0, basic grant", addr: "0.0.0.0:7654", grantsConfigured: true, wantErr: false},
		{name: "LAN, mtls grant", addr: "192.168.1.24:7654", grantsConfigured: true, wantErr: false},
		{name: "LAN, trust-header grant", addr: "192.168.1.24:7654", grantsConfigured: true, wantErr: false},
		{name: "LAN, insecure-no-auth", addr: "192.168.1.24:7654", insecureNoAuth: true, wantErr: false},
		{name: "0.0.0.0, insecure-no-auth", addr: "0.0.0.0:7654", insecureNoAuth: true, wantErr: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := BindGuard(tt.addr, tt.isUnixSocket, "bash", tt.grantsConfigured, tt.insecureNoAuth)
			if (err != nil) != tt.wantErr {
				t.Fatalf("BindGuard(%q) error = %v, wantErr %v", tt.addr, err, tt.wantErr)
			}
			if err != nil {
				want := fmt.Sprintf(bindGuardMessage, tt.addr, "bash")
				if err.Error() != want {
					t.Fatalf("BindGuard(%q) message mismatch:\ngot:  %s\nwant: %s", tt.addr, err.Error(), want)
				}
			}
		})
	}
}
