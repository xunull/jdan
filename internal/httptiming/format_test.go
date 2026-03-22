package httptiming

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func fixedResult() Result {
	return Result{
		URL:              "https://example.com",
		StatusCode:       200,
		DNSLookup:        23*time.Millisecond + 450*time.Microsecond,
		TCPConnect:       45*time.Millisecond + 120*time.Microsecond,
		TLSHandshake:     89*time.Millisecond + 340*time.Microsecond,
		ServerProcessing: 120*time.Millisecond + 560*time.Microsecond,
		ContentTransfer:  15*time.Millisecond + 230*time.Microsecond,
		Total:            293*time.Millisecond + 700*time.Microsecond,
	}
}

func TestFormatText_single(t *testing.T) {
	text := FormatText([]Result{fixedResult()})
	for _, want := range []string{"DNS 查询:", "TCP 连接:", "TLS 握手:", "服务端处理:", "内容传输:", "总耗时:", "200"} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q in:\n%s", want, text)
		}
	}
	if strings.Contains(text, "#1") {
		t.Error("single result should not have #1 label")
	}
}

func TestFormatText_multi(t *testing.T) {
	text := FormatText([]Result{fixedResult(), fixedResult()})
	for _, want := range []string{"#1", "#2", "平均值"} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q in:\n%s", want, text)
		}
	}
}

func TestFormatJSON_single(t *testing.T) {
	out, err := FormatJSON([]Result{fixedResult()})
	if err != nil {
		t.Fatal(err)
	}
	var obj jsonResult
	if err := json.Unmarshal([]byte(out), &obj); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if obj.StatusCode != 200 {
		t.Fatalf("status = %d", obj.StatusCode)
	}
	if obj.TotalMs < 290 {
		t.Fatalf("total = %f", obj.TotalMs)
	}
}

func TestFormatJSON_multi(t *testing.T) {
	out, err := FormatJSON([]Result{fixedResult(), fixedResult()})
	if err != nil {
		t.Fatal(err)
	}
	var obj jsonOutput
	if err := json.Unmarshal([]byte(out), &obj); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(obj.Results) != 2 {
		t.Fatalf("results len = %d", len(obj.Results))
	}
	if obj.Average == nil {
		t.Fatal("average is nil")
	}
}
