package middleware

import "testing"

func TestAnonymizeIP(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{"IPv4 basic", "192.168.1.42", "192.168.1.0"},
		{"IPv4 already zero", "10.0.0.0", "10.0.0.0"},
		{"IPv4 high octet", "255.255.255.255", "255.255.255.0"},
		{"IPv6 full", "2001:0db8:0000:0000:0000:0000:0000:0001", "2001:db8::"},
		{"IPv6 short", "2001:db8::1", "2001:db8::"},
		{"IPv6 loopback", "::1", "::"},
		{"invalid string", "not-an-ip", "not-an-ip"},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AnonymizeIP(tt.raw)
			if got != tt.want {
				t.Errorf("AnonymizeIP(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}
