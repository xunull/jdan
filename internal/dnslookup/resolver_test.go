package dnslookup

import "testing"

func TestEnsurePort(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"", "8.8.8.8:53"},
		{"8.8.8.8", "8.8.8.8:53"},
		{"8.8.8.8:53", "8.8.8.8:53"},
		{"8.8.8.8:5353", "8.8.8.8:5353"},
		{"dns.google", "dns.google:53"},
		{"dns.google:53", "dns.google:53"},
		{"::1", "[::1]:53"},
		{"2001:db8::1", "[2001:db8::1]:53"},
		{"[::1]:53", "[::1]:53"},
		{"[2001:db8::1]:5353", "[2001:db8::1]:5353"},
		{"https://dns.google/dns-query", "https://dns.google/dns-query"},
		{"HTTPS://dns.google/dns-query", "HTTPS://dns.google/dns-query"},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			if got := ensurePort(tc.in); got != tc.want {
				t.Errorf("ensurePort(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
