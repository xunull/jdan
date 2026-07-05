package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func runIPAgg(t *testing.T, stdin string, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	cmd := newIPAggregateCommand(ipCmdDeps{out: &out})
	cmd.SetArgs(args)
	cmd.SetIn(strings.NewReader(stdin))
	err := cmd.Execute()
	return out.String(), err
}

func runRangeCIDR(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	cmd := newIPRangeCIDRCommand(ipCmdDeps{out: &out})
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

func TestIPAggregate_Args(t *testing.T) {
	out, err := runIPAgg(t, "", "10.0.0.0/25", "10.0.0.128/25")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "10.0.0.0/24") || !strings.Contains(out, "(2 in → 1 out)") {
		t.Errorf("聚合输出不对:\n%s", out)
	}
}

func TestIPAggregate_BareIP(t *testing.T) {
	// 裸 IP 当 /32；两个相邻 → /31
	out, err := runIPAgg(t, "", "10.0.0.0", "10.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "10.0.0.0/31") {
		t.Errorf("裸 IP 聚合不对:\n%s", out)
	}
}

func TestIPAggregate_Stdin(t *testing.T) {
	out, err := runIPAgg(t, "10.0.0.0/25\n10.0.0.128/25\n")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "10.0.0.0/24") {
		t.Errorf("stdin 聚合不对:\n%s", out)
	}
}

func TestIPAggregate_JSON(t *testing.T) {
	out, err := runIPAgg(t, "", "--json", "10.0.0.0/25", "10.0.0.128/25")
	if err != nil {
		t.Fatal(err)
	}
	var v struct {
		In    int
		Out   int
		Cidrs []string
	}
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		t.Fatalf("非法 json:\n%s", out)
	}
	if v.In != 2 || v.Out != 1 || len(v.Cidrs) != 1 || v.Cidrs[0] != "10.0.0.0/24" {
		t.Errorf("json 不对: %+v", v)
	}
}

func TestIPAggregate_Invalid(t *testing.T) {
	if _, err := runIPAgg(t, "", "not-an-ip"); err == nil {
		t.Error("非法输入应报错")
	}
}

func TestRangeCIDR_TwoArgs(t *testing.T) {
	out, err := runRangeCIDR(t, "192.168.1.5", "192.168.1.20")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"192.168.1.5/32", "192.168.1.6/31", "192.168.1.8/29", "192.168.1.16/30", "192.168.1.20/32", "(5 CIDRs)"} {
		if !strings.Contains(out, want) {
			t.Errorf("缺 %q:\n%s", want, out)
		}
	}
}

func TestRangeCIDR_SingleArg(t *testing.T) {
	out, err := runRangeCIDR(t, "192.168.1.5-192.168.1.20")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "192.168.1.8/29") || !strings.Contains(out, "(5 CIDRs)") {
		t.Errorf("单参数写法不对:\n%s", out)
	}
}

func TestRangeCIDR_FamilyMismatch(t *testing.T) {
	if _, err := runRangeCIDR(t, "10.0.0.1", "2001:db8::1"); err == nil {
		t.Error("跨族应报错")
	}
}

func TestRangeCIDR_JSON(t *testing.T) {
	out, err := runRangeCIDR(t, "--json", "10.0.0.0", "10.0.0.255")
	if err != nil {
		t.Fatal(err)
	}
	var v struct {
		Start, End string
		Count      int
		Cidrs      []string
	}
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		t.Fatalf("非法 json:\n%s", out)
	}
	if v.Count != 1 || len(v.Cidrs) != 1 || v.Cidrs[0] != "10.0.0.0/24" {
		t.Errorf("json 不对: %+v", v)
	}
}
