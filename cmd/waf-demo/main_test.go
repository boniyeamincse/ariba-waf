package main

import "testing"

func TestValidateLoopbackAddress(t *testing.T) {
	tests := []struct {
		name    string
		address string
		wantErr bool
	}{
		{name: "IPv4 loopback", address: "127.0.0.1:18080"},
		{name: "IPv6 loopback", address: "[::1]:18080"},
		{name: "all interfaces", address: "0.0.0.0:18080", wantErr: true},
		{name: "private interface", address: "192.168.1.10:18080", wantErr: true},
		{name: "hostname", address: "localhost:18080", wantErr: true},
		{name: "malformed address", address: "not-an-address", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := validateLoopbackAddress(tt.address)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateLoopbackAddress() error = %v, wantErr %t", err, tt.wantErr)
			}
			if err == nil && got != tt.address {
				t.Errorf("address = %q, want %q", got, tt.address)
			}
		})
	}
}