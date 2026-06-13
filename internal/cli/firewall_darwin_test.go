//go:build darwin

package cli

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestMacOSFirewallHint_StateEnabled(t *testing.T) {
	orig := firewallCheckCmd
	defer func() { firewallCheckCmd = orig }()

	firewallCheckCmd = func(ctx context.Context) ([]byte, error) {
		return []byte("Firewall is enabled. (State = 1)\n"), nil
	}

	hint := macOSFirewallHint()
	if hint == "" {
		t.Fatal("expected hint when firewall enabled")
	}
	for _, want := range []string{"firewall", "LAN", "README"} {
		if !strings.Contains(hint, want) {
			t.Errorf("hint missing %q: %s", want, hint)
		}
	}
}

func TestMacOSFirewallHint_StateDisabled(t *testing.T) {
	orig := firewallCheckCmd
	defer func() { firewallCheckCmd = orig }()

	firewallCheckCmd = func(ctx context.Context) ([]byte, error) {
		return []byte("Firewall is disabled. (State = 0)\n"), nil
	}

	if hint := macOSFirewallHint(); hint != "" {
		t.Errorf("State=0 should produce no hint, got: %s", hint)
	}
}

func TestMacOSFirewallHint_CmdError(t *testing.T) {
	orig := firewallCheckCmd
	defer func() { firewallCheckCmd = orig }()

	firewallCheckCmd = func(ctx context.Context) ([]byte, error) {
		return nil, errors.New("permission denied")
	}

	if hint := macOSFirewallHint(); hint != "" {
		t.Errorf("error should produce empty hint silently, got: %s", hint)
	}
}

func TestMacOSFirewallHint_GarbageOutput(t *testing.T) {
	orig := firewallCheckCmd
	defer func() { firewallCheckCmd = orig }()

	firewallCheckCmd = func(ctx context.Context) ([]byte, error) {
		return []byte("some unexpected garbage"), nil
	}

	if hint := macOSFirewallHint(); hint != "" {
		t.Errorf("garbage output should produce no hint, got: %s", hint)
	}
}
