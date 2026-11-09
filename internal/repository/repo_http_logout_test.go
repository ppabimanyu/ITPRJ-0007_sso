package repository

import (
	"context"
	"net/netip"
	"testing"
)

func TestIsPublicIP(t *testing.T) {
	cases := []struct {
		address string
		public  bool
	}{
		{"8.8.8.8", true},
		{"1.1.1.1", true},
		{"127.0.0.1", false},
		{"::1", false},
		{"10.1.2.3", false},
		{"172.16.0.1", false},
		{"192.168.1.1", false},
		{"169.254.169.254", false},
		{"100.64.0.1", false},
		{"0.0.0.0", false},
		{"fc00::1", false},
		{"fe80::1", false},
	}
	for _, tc := range cases {
		address := netip.MustParseAddr(tc.address)
		if got := isPublicIP(address); got != tc.public {
			t.Errorf("isPublicIP(%s) = %v, want %v", tc.address, got, tc.public)
		}
	}
}

func TestValidateLogoutURIRejectsUnsafeTargets(t *testing.T) {
	ctx := context.Background()
	cases := []string{
		"http://8.8.8.8/backchannel",
		"https://127.0.0.1/backchannel",
		"https://10.0.0.5/backchannel",
		"https://169.254.169.254/latest",
		"https://user:pass@8.8.8.8/backchannel",
	}
	for _, raw := range cases {
		if err := validateLogoutURI(ctx, raw); err == nil {
			t.Errorf("validateLogoutURI(%q) accepted an unsafe target", raw)
		}
	}
}
