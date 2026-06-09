package dnslookup

import (
	"encoding/json"
	"strings"
	"testing"
)

func sampleResult() *Result {
	return &Result{
		Domain:      "example.com",
		Server:      "8.8.8.8:53",
		QueryTimeMs: 23,
		Results: []TypeResult{
			{Type: "A", Rcode: "NOERROR", TTL: 3600, Values: []string{"93.184.216.34"}},
			{Type: "AAAA", Rcode: "NOERROR", TTL: 3600, Values: []string{"2606:2800:220:1:248:1893:25c8:1946"}},
			{Type: "MX", Rcode: "NOERROR", Values: []string{}},                              // 空记录
			{Type: "TXT", Rcode: "NOERROR", TTL: 600, Values: []string{`"v=spf1 -all"`}},    // 含引号
			{Type: "CNAME", Err: "TIMEOUT", Values: []string{}},                             // 网络错误
			{Type: "NS", Rcode: "NOERROR", TTL: 86400, Values: []string{"a.iana-servers.net.", "b.iana-servers.net."}}, // 多值
		},
	}
}

func TestFormatText_HeaderShowsDomainAndServer(t *testing.T) {
	out := FormatText(sampleResult())
	if !strings.HasPrefix(out, "example.com — via 8.8.8.8:53\n") {
		t.Errorf("missing header, got first line: %q", strings.SplitN(out, "\n", 2)[0])
	}
}

func TestFormatText_HasColumnHeaders(t *testing.T) {
	out := FormatText(sampleResult())
	if !strings.Contains(out, "TYPE") || !strings.Contains(out, "TTL") || !strings.Contains(out, "VALUE") {
		t.Errorf("missing column headers in:\n%s", out)
	}
}

func TestFormatText_TimeoutRowHasWarnMark(t *testing.T) {
	out := FormatText(sampleResult())
	if !strings.Contains(out, "CNAME") {
		t.Errorf("CNAME row missing")
	}
	if !strings.Contains(out, warnMark) || !strings.Contains(out, "TIMEOUT") {
		t.Errorf("expected ⚠ + TIMEOUT, got:\n%s", out)
	}
}

func TestFormatText_EmptyAnswerShowsPlaceholder(t *testing.T) {
	out := FormatText(sampleResult())
	if !strings.Contains(out, "(no records)") {
		t.Errorf("expected '(no records)' for empty MX answer, got:\n%s", out)
	}
}

func TestFormatText_MultiValueIndented(t *testing.T) {
	out := FormatText(sampleResult())
	// 第二个 NS 值应该出现在自己的行（不带 TYPE/TTL 列）。tabwriter 会用空格填齐，
	// 但 "b.iana-servers.net." 这行不应该以 "NS" 开头。
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "b.iana-servers.net.") {
			trimmed := strings.TrimLeft(line, " \t")
			if strings.HasPrefix(trimmed, "NS") {
				t.Errorf("second NS value should be on its own continuation line, got: %q", line)
			}
		}
	}
}

func TestFormatText_TXTQuotesPreserved(t *testing.T) {
	out := FormatText(sampleResult())
	if !strings.Contains(out, `"v=spf1 -all"`) {
		t.Errorf("TXT quoted string should be preserved, got:\n%s", out)
	}
}

func TestFormatText_NXDOMAINMarkedWarn(t *testing.T) {
	res := &Result{
		Domain: "doesnotexist.example",
		Server: "8.8.8.8:53",
		Results: []TypeResult{
			{Type: "A", Rcode: "NXDOMAIN", Values: []string{}},
		},
	}
	out := FormatText(res)
	if !strings.Contains(out, warnMark) || !strings.Contains(out, "NXDOMAIN") {
		t.Errorf("NXDOMAIN should show ⚠ + NXDOMAIN, got:\n%s", out)
	}
}

func TestFormatShort_OnlyValues(t *testing.T) {
	out := FormatShort(sampleResult())
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	want := []string{
		"93.184.216.34",
		"2606:2800:220:1:248:1893:25c8:1946",
		`"v=spf1 -all"`,
		"a.iana-servers.net.",
		"b.iana-servers.net.",
	}
	if len(lines) != len(want) {
		t.Fatalf("expected %d lines, got %d:\n%s", len(want), len(lines), out)
	}
	for i, w := range want {
		if lines[i] != w {
			t.Errorf("line %d: got %q, want %q", i, lines[i], w)
		}
	}
}

func TestFormatShort_SkipsEmptyAndFailed(t *testing.T) {
	// MX 空 + CNAME timeout 都不应该出现在 --short 输出里
	out := FormatShort(sampleResult())
	if strings.Contains(out, "TIMEOUT") {
		t.Errorf("--short should not show errors, got:\n%s", out)
	}
	if strings.Contains(out, "(no records)") {
		t.Errorf("--short should not show empty placeholders, got:\n%s", out)
	}
}

func TestFormatVerbose_IncludesQueryTime(t *testing.T) {
	out := FormatVerbose(sampleResult())
	if !strings.Contains(out, "query time: 23 ms") {
		t.Errorf("missing query time, got:\n%s", out)
	}
}

func TestFormatVerbose_HasRcodeColumn(t *testing.T) {
	out := FormatVerbose(sampleResult())
	if !strings.Contains(out, "RCODE") {
		t.Errorf("verbose should have RCODE column, got:\n%s", out)
	}
}

func TestFormatJSON_ValidAndCompleteFields(t *testing.T) {
	out, err := FormatJSON(sampleResult())
	if err != nil {
		t.Fatalf("FormatJSON error: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	for _, k := range []string{"domain", "server", "query_time_ms", "results"} {
		if _, ok := parsed[k]; !ok {
			t.Errorf("missing field %q", k)
		}
	}
	results := parsed["results"].([]any)
	if len(results) != 6 {
		t.Errorf("expected 6 result entries, got %d", len(results))
	}
}

func TestFormatJSON_EmptyValuesRendersAsEmptyArray(t *testing.T) {
	// 空 record 的 values 字段应该是 [] 而不是 null（脚本消费者友好）
	res := &Result{
		Domain: "x", Server: "y",
		Results: []TypeResult{{Type: "MX", Rcode: "NOERROR", Values: []string{}}},
	}
	out, _ := FormatJSON(res)
	if !strings.Contains(out, `"values": []`) {
		t.Errorf("empty values should marshal as [], got:\n%s", out)
	}
}

func TestFormatText_DisplayNameOverridesHeaderDomain(t *testing.T) {
	// reverse 场景：Domain 是 arpa 形式，DisplayName 是原始 IP，顶部应显示 IP。
	res := &Result{
		Domain:      "8.8.8.8.in-addr.arpa.",
		DisplayName: "8.8.8.8",
		Server:      "1.1.1.1:53",
		Results: []TypeResult{
			{Type: "PTR", Rcode: "NOERROR", TTL: 300, Values: []string{"dns.google."}},
		},
	}
	out := FormatText(res)
	if !strings.HasPrefix(out, "8.8.8.8 — via 1.1.1.1:53\n") {
		t.Errorf("expected header 8.8.8.8 — via, got first line: %q", strings.SplitN(out, "\n", 2)[0])
	}
	if strings.Contains(out, "in-addr.arpa") {
		t.Errorf("arpa form leaked into text output: %s", out)
	}
}

func TestFormatText_NoDisplayNameFallsBackToDomain(t *testing.T) {
	// lookup 场景：DisplayName 为空，顶部回退到 Domain（保持现有行为）。
	res := &Result{
		Domain: "example.com",
		Server: "8.8.8.8:53",
		Results: []TypeResult{
			{Type: "A", Rcode: "NOERROR", TTL: 60, Values: []string{"1.2.3.4"}},
		},
	}
	out := FormatText(res)
	if !strings.HasPrefix(out, "example.com — via 8.8.8.8:53\n") {
		t.Errorf("expected fallback to Domain, got first line: %q", strings.SplitN(out, "\n", 2)[0])
	}
}

func TestFormatVerbose_DisplayNameOverridesHeaderDomain(t *testing.T) {
	res := &Result{
		Domain:      "8.8.8.8.in-addr.arpa.",
		DisplayName: "8.8.8.8",
		Server:      "1.1.1.1:53",
		QueryTimeMs: 12,
		Results: []TypeResult{
			{Type: "PTR", Rcode: "NOERROR", TTL: 300, Values: []string{"dns.google."}},
		},
	}
	out := FormatVerbose(res)
	if !strings.HasPrefix(out, "8.8.8.8 — via 1.1.1.1:53\n") {
		t.Errorf("verbose header should use DisplayName, got: %q", strings.SplitN(out, "\n", 2)[0])
	}
}

func TestFormatJSON_DisplayNameOmittedWhenEmpty(t *testing.T) {
	// lookup 场景：JSON 不应该多一个 display_name 字段
	res := &Result{
		Domain: "x", Server: "y",
		Results: []TypeResult{{Type: "A", Rcode: "NOERROR", TTL: 60, Values: []string{"1.2.3.4"}}},
	}
	out, _ := FormatJSON(res)
	if strings.Contains(out, "display_name") {
		t.Errorf("display_name should be omitted when empty, got:\n%s", out)
	}
}

func TestFormatJSON_DisplayNameIncludedWhenSet(t *testing.T) {
	res := &Result{
		Domain: "8.8.8.8.in-addr.arpa.", DisplayName: "8.8.8.8", Server: "1.1.1.1:53",
		Results: []TypeResult{{Type: "PTR", Rcode: "NOERROR", TTL: 300, Values: []string{"dns.google."}}},
	}
	out, _ := FormatJSON(res)
	if !strings.Contains(out, `"display_name": "8.8.8.8"`) {
		t.Errorf("display_name should appear in JSON, got:\n%s", out)
	}
	if !strings.Contains(out, `"domain": "8.8.8.8.in-addr.arpa."`) {
		t.Errorf("domain should still carry arpa form for scripts wanting raw query, got:\n%s", out)
	}
}

func TestFormatJSON_ErrorFieldOmittedWhenEmpty(t *testing.T) {
	res := &Result{
		Domain: "x", Server: "y",
		Results: []TypeResult{{Type: "A", Rcode: "NOERROR", TTL: 100, Values: []string{"1.2.3.4"}}},
	}
	out, _ := FormatJSON(res)
	if strings.Contains(out, `"error"`) {
		t.Errorf("error field should be omitted when empty, got:\n%s", out)
	}
}
