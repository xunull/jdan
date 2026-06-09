package dnslookup

import (
	"encoding/json"
	"fmt"
	"strings"
	"text/tabwriter"
)

// 失败行的视觉标记（U+26A0 警告符）。
const warnMark = "⚠"

// headerDomain 决定顶部一行展示的 domain 标签：reverse 查询用 DisplayName
// 显示原始 IP；lookup 查询 DisplayName 为空，回退到 Domain。
func headerDomain(res *Result) string {
	if res.DisplayName != "" {
		return res.DisplayName
	}
	return res.Domain
}

// FormatText 渲染默认三列输出：TYPE / TTL / VALUE。多值条目在后续行只填 VALUE 列。
//
// 顶部一行 `domain — via server`，便于用户确认查询是否走了期望的 resolver。
func FormatText(res *Result) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s — via %s\n\n", headerDomain(res), res.Server)

	w := tabwriter.NewWriter(&b, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "TYPE\tTTL\tVALUE")

	for _, tr := range res.Results {
		writeTextRow(w, tr)
	}
	_ = w.Flush()
	return b.String()
}

func writeTextRow(w *tabwriter.Writer, tr TypeResult) {
	switch {
	case tr.Err != "":
		fmt.Fprintf(w, "%s\t%s\t%s\n", tr.Type, warnMark, tr.Err)
	case tr.Rcode != "NOERROR":
		fmt.Fprintf(w, "%s\t%s\t%s\n", tr.Type, warnMark, tr.Rcode)
	case len(tr.Values) == 0:
		fmt.Fprintf(w, "%s\t—\t(no records)\n", tr.Type)
	default:
		fmt.Fprintf(w, "%s\t%d\t%s\n", tr.Type, tr.TTL, tr.Values[0])
		for _, v := range tr.Values[1:] {
			fmt.Fprintf(w, "\t\t%s\n", v)
		}
	}
}

// FormatShort 输出 dig +short 风格：仅值，一行一条。无值的 type 被跳过。
//
// 用于 `IP=$(jdan dns lookup example.com -t A --short)` 这种脚本场景。
func FormatShort(res *Result) string {
	var b strings.Builder
	for _, tr := range res.Results {
		for _, v := range tr.Values {
			fmt.Fprintln(&b, v)
		}
	}
	return b.String()
}

// FormatVerbose 在 text 输出之上追加 query time 等元数据，并把 rcode 单独列一列。
func FormatVerbose(res *Result) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s — via %s\n", headerDomain(res), res.Server)
	fmt.Fprintf(&b, "query time: %d ms\n\n", res.QueryTimeMs)

	w := tabwriter.NewWriter(&b, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "TYPE\tTTL\tRCODE\tVALUE")

	for _, tr := range res.Results {
		writeVerboseRow(w, tr)
	}
	_ = w.Flush()
	return b.String()
}

func writeVerboseRow(w *tabwriter.Writer, tr TypeResult) {
	rcode := tr.Rcode
	if tr.Err != "" {
		rcode = tr.Err
	}
	switch {
	case tr.Err != "" || tr.Rcode != "NOERROR":
		fmt.Fprintf(w, "%s\t—\t%s\t%s\n", tr.Type, rcode, warnMark)
	case len(tr.Values) == 0:
		fmt.Fprintf(w, "%s\t—\t%s\t(no records)\n", tr.Type, rcode)
	default:
		fmt.Fprintf(w, "%s\t%d\t%s\t%s\n", tr.Type, tr.TTL, rcode, tr.Values[0])
		for _, v := range tr.Values[1:] {
			fmt.Fprintf(w, "\t\t\t%s\n", v)
		}
	}
}

// FormatJSON 输出完整 metadata。Values 字段被显式初始化为 []，因此空记录渲染为 [] 而非 null。
func FormatJSON(res *Result) (string, error) {
	out, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		return "", err
	}
	return string(out), nil
}
