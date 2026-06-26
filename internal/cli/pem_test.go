package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/xunull/jdan/internal/certgen"
)

func runPem(t *testing.T, stdin string, args ...string) (string, error) {
	t.Helper()
	var buf bytes.Buffer
	deps := pemCmdDeps{out: &buf}
	if stdin != "" {
		deps.in = strings.NewReader(stdin)
	}
	cmd := newPemCommand(deps)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return buf.String(), err
}

func certKeyPEM(t *testing.T) string {
	t.Helper()
	r, err := certgen.GenerateSelfSigned(certgen.Options{CommonName: "example.com", KeyType: certgen.KeyEC, Days: 365})
	if err != nil {
		t.Fatal(err)
	}
	return string(r.CertPEM) + string(r.KeyPEM)
}

func TestPemCmd_Text(t *testing.T) {
	out, err := runPem(t, certKeyPEM(t))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "CERTIFICATE") || !strings.Contains(out, "example.com") {
		t.Errorf("text output wrong:\n%s", out)
	}
	if !strings.Contains(out, "私钥与证书匹配") {
		t.Errorf("expected key-cert match line:\n%s", out)
	}
}

func TestPemCmd_JSON(t *testing.T) {
	out, err := runPem(t, certKeyPEM(t), "--json")
	if err != nil {
		t.Fatal(err)
	}
	var v map[string]any
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		t.Fatalf("--json should emit valid JSON: %v\n%s", err, out)
	}
}

func TestPemCmd_NoPEM(t *testing.T) {
	if _, err := runPem(t, "not pem"); err == nil {
		t.Error("non-PEM input should error")
	}
}

func TestPemCmd_NoPrivateKeyLeak(t *testing.T) {
	out, err := runPem(t, certKeyPEM(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range strings.Fields(out) {
		if len(field) >= 60 && isB64Field(field) {
			t.Errorf("possible private key leak in CLI output: %s", field)
		}
	}
}

func isB64Field(s string) bool {
	for _, r := range s {
		if !((r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '+' || r == '/' || r == '=') {
			return false
		}
	}
	return true
}
