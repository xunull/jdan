//go:build darwin

package sysprobe

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestMacFirewallHint_StateEnabled(t *testing.T) {
	orig := FirewallCheckCmd
	defer func() { FirewallCheckCmd = orig }()

	FirewallCheckCmd = func(ctx context.Context) ([]byte, error) {
		return []byte("Firewall is enabled. (State = 1)\n"), nil
	}

	hint := MacFirewallHint()
	if hint == "" {
		t.Fatal("expected hint when firewall enabled")
	}
	for _, want := range []string{"firewall", "LAN", "README"} {
		if !strings.Contains(hint, want) {
			t.Errorf("hint missing %q: %s", want, hint)
		}
	}
	if state := MacFirewallState(); state != FirewallEnabled {
		t.Errorf("state should be FirewallEnabled, got %v", state)
	}
}

func TestMacFirewallHint_StateBlockAll(t *testing.T) {
	orig := FirewallCheckCmd
	defer func() { FirewallCheckCmd = orig }()

	FirewallCheckCmd = func(ctx context.Context) ([]byte, error) {
		return []byte("Firewall is enabled. (State = 2)\n"), nil
	}

	// State = 2 (block all incoming) 也应当被视为 enabled
	if state := MacFirewallState(); state != FirewallEnabled {
		t.Errorf("State=2 should still be FirewallEnabled, got %v", state)
	}
	if MacFirewallHint() == "" {
		t.Error("State=2 should produce hint")
	}
}

func TestMacFirewallHint_StateDisabled(t *testing.T) {
	orig := FirewallCheckCmd
	defer func() { FirewallCheckCmd = orig }()

	FirewallCheckCmd = func(ctx context.Context) ([]byte, error) {
		return []byte("Firewall is disabled. (State = 0)\n"), nil
	}

	if hint := MacFirewallHint(); hint != "" {
		t.Errorf("State=0 should produce no hint, got: %s", hint)
	}
	if state := MacFirewallState(); state != FirewallDisabled {
		t.Errorf("state should be FirewallDisabled, got %v", state)
	}
}

func TestMacFirewallHint_CmdError(t *testing.T) {
	orig := FirewallCheckCmd
	defer func() { FirewallCheckCmd = orig }()

	FirewallCheckCmd = func(ctx context.Context) ([]byte, error) {
		return nil, errors.New("permission denied")
	}

	if hint := MacFirewallHint(); hint != "" {
		t.Errorf("error should produce empty hint silently, got: %s", hint)
	}
	if state := MacFirewallState(); state != FirewallUnknown {
		t.Errorf("error should yield FirewallUnknown, got %v", state)
	}
}

func TestMacFirewallHint_GarbageOutput(t *testing.T) {
	orig := FirewallCheckCmd
	defer func() { FirewallCheckCmd = orig }()

	FirewallCheckCmd = func(ctx context.Context) ([]byte, error) {
		return []byte("some unexpected garbage"), nil
	}

	if hint := MacFirewallHint(); hint != "" {
		t.Errorf("garbage output should produce no hint, got: %s", hint)
	}
	if state := MacFirewallState(); state != FirewallUnknown {
		t.Errorf("garbage should yield FirewallUnknown, got %v", state)
	}
}
