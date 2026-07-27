//go:build ignore

// gen_anchors.go 从国立天文台（NAOJ）暦計算室抓 1900–2100 全部二十四节气，
// 生成 testdata/terms_1900_2100.tsv 供全量对拍。
//
// 用法：
//
//	go run internal/jieqix/_tools/gen_anchors.go -from 1900 -to 2100 \
//	    > internal/jieqix/testdata/terms_1900_2100.tsv
//
// ⚠️ 这是**一次性**工具，产物已提交进仓库，正常开发不需要重跑。
// 它要向 NAOJ 的公开服务发 201 次请求；请求间默认停 1.5 秒，别把它调小，
// 也别为了「顺手刷新一下」而反复跑——对方是公益的天文研究机构。
//
// 为什么用 NAOJ 而不是另一个第三方实现：
//   - 它是权威源本身。对拍第三方实现只能证明「两个实现一致」，
//     而两个实现完全可能同源（都出自 VSOP87 + Meeus），一起错。
//   - 它的远期 ΔT 用 Stephenson 等 2020 模型，与本实现的 Espenak–Meeus
//     是**不同的**模型。这意味着 2050 年后的偏差是真实的模型分歧，
//     不是巧合的一致——对拍才有意义。
//   - 支持 -2999~2999 年，且能直接指定输出时区（本工具取 UT）。
//
// 接口（POST，application/x-www-form-urlencoded）：
//
//	https://eco.mtk.nao.ac.jp/cgi-bin/koyomi/cande/phenomena_sy.cgi
//	year=<Y>  lst=0（世界时）  phenom=50（二十四節気）
//	jg=2（1582-10-15 起格里高利历）  dtm=0（ΔT 用其默认模型）
//
// 响应是 EUC-JP 编码的 HTML。节气名用日式写法（啓蟄/処暑/小満/穀雨/芒種），
// 本工具转成简体中文，与 internal/jieqix 的 termDefs 对齐。
package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const endpoint = "https://eco.mtk.nao.ac.jp/cgi-bin/koyomi/cande/phenomena_sy.cgi"

// 日式节气名 → 简体中文。其余 19 个两边写法相同。
var jp2cn = map[string]string{
	"啓蟄": "惊蛰", "処暑": "处暑", "小満": "小满", "穀雨": "谷雨", "芒種": "芒种",
}

var (
	rowRe = regexp.MustCompile(
		`(\d{4})/(\d{2})/(\d{2})\s+(\d{2}):(\d{2})\s+太陽\s+二十四節気\s+(\S+?)\(黄経(-?\d+)`)
	dtRe = regexp.MustCompile(`ΔＴ＝\s*([\-0-9.]+)\s*s`)
	tzRe = regexp.MustCompile(`標準時:\s*UT([+\-][\d.]+)\s*h`)

	// 表格字段之间夹着 <td> 之类的标签，正则必须跑在剥完标签的纯文本上。
	// 直接匹配原始 HTML 会一条都对不上——而且是静默对不上，
	// 所以下面 fetchYear 里有「解析出的条数必须是 24」这道硬检查。
	tagRe   = regexp.MustCompile(`(?s)<script.*?</script>|<[^>]+>`)
	spaceRe = regexp.MustCompile(`\s+`)
)

// plainText 把 HTML 剥成一行纯文本，字段间保证有空白分隔。
func plainText(html string) string {
	return spaceRe.ReplaceAllString(tagRe.ReplaceAllString(html, " "), " ")
}

func main() {
	from := flag.Int("from", 1900, "起始年")
	to := flag.Int("to", 2100, "结束年")
	delay := flag.Duration("delay", 1500*time.Millisecond, "请求间隔（别调小）")
	flag.Parse()

	fmt.Printf("# 二十四节气全量对拍数据（第二层验证）\n#\n")
	fmt.Printf("# 时间基准：UT (UTC+0)\n")
	fmt.Printf("# 来源：国立天文台（NAOJ）暦計算室「二十四節気・雑節 長期版」\n")
	fmt.Printf("#   %s\n", endpoint)
	fmt.Printf("#   POST year=<Y>&lst=0&phenom=50&jg=2&dtm=0\n")
	fmt.Printf("# 生成：go run internal/jieqix/_tools/gen_anchors.go -from %d -to %d\n", *from, *to)
	fmt.Printf("#\n# 与第一层（testdata/solar_terms_ut.tsv，144 条手工锚点 + AstroPixels\n")
	fmt.Printf("# 交叉验证）分工：第一层验算法整体对不对，本表验有没有只在某些\n")
	fmt.Printf("# 年份区间显形的系统性偏移。\n#\n")
	fmt.Printf("# 列：year  term_cn  ecliptic_longitude_deg  ut_datetime\n")

	total := 0
	for y := *from; y <= *to; y++ {
		rows, dt, err := fetchYearRetry(y)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%d: %v\n", y, err)
			os.Exit(1)
		}
		if len(rows) != 24 {
			fmt.Fprintf(os.Stderr, "%d: 只解析出 %d 个节气（应为 24），页面格式可能变了\n", y, len(rows))
			os.Exit(1)
		}
		for _, r := range rows {
			fmt.Println(r)
		}
		total += len(rows)
		fmt.Fprintf(os.Stderr, "\r%d  (ΔT=%ss, 累计 %d 条)   ", y, dt, total)
		if y != *to {
			time.Sleep(*delay)
		}
	}
	fmt.Fprintf(os.Stderr, "\n完成：%d 年 %d 条\n", *to-*from+1, total)
}

// 201 次请求跑下来，中途断连是常态（实测第 2 年就 EOF）。
// 每年重试 3 次，退避 2/4/8 秒；关掉 keep-alive，让每次请求各自建连——
// 长连接被对端悄悄关掉正是 EOF 的来源。
var client = &http.Client{
	Timeout:   30 * time.Second,
	Transport: &http.Transport{DisableKeepAlives: true},
}

func fetchYearRetry(year int) ([]string, string, error) {
	var lastErr error
	for attempt := 1; attempt <= 3; attempt++ {
		rows, dt, err := fetchYear(year)
		if err == nil {
			return rows, dt, nil
		}
		lastErr = err
		back := time.Duration(1<<attempt) * time.Second
		fmt.Fprintf(os.Stderr, "\n%d 第 %d 次失败（%v），%v 后重试\n", year, attempt, err, back)
		time.Sleep(back)
	}
	return nil, "", fmt.Errorf("重试 3 次仍失败：%w", lastErr)
}

func fetchYear(year int) ([]string, string, error) {
	form := url.Values{
		"year":   {strconv.Itoa(year)},
		"lst":    {"0"}, // 世界时
		"phenom": {"50"},
		"jg":     {"2"},
		"dtm":    {"0"},
		"cal":    {"0"},
	}
	resp, err := client.PostForm(endpoint, form)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}
	// EUC-JP：本工具只需要匹配 ASCII 数字与几个固定汉字串，
	// 而正则里的汉字是 UTF-8，所以要先转码。EUC-JP → UTF-8 用最小实现，
	// 避免为一个一次性工具引入 golang.org/x/text。
	body := plainText(decodeEUCJP(raw))

	if m := tzRe.FindStringSubmatch(body); m == nil || m[1] != "+0" {
		return nil, "", fmt.Errorf("响应时区不是 UT+0（拿到 %v），lst 参数可能失效了", m)
	}
	dt := "?"
	if m := dtRe.FindStringSubmatch(body); m != nil {
		dt = m[1]
	}

	var out []string
	for _, m := range rowRe.FindAllStringSubmatch(body, -1) {
		name := m[6]
		if cn, ok := jp2cn[name]; ok {
			name = cn
		}
		// NAOJ 按日式写法把恰好落在午夜的时刻记作「24:00」而不是次日 00:00，
		// 那不是合法的 RFC3339 小时。规范化成次日 00:00。
		// 201 年里只有个位数条，但不处理会让整个 fixture 解析失败。
		ts, err := time.Parse("2006-01-02 15:04", fmt.Sprintf("%s-%s-%s %s:%s",
			m[1], m[2], m[3], m[4], m[5]))
		if err != nil {
			if m[4] != "24" {
				return nil, "", fmt.Errorf("无法解析时刻 %s:%s", m[4], m[5])
			}
			ts, err = time.Parse("2006-01-02 15:04", fmt.Sprintf("%s-%s-%s 00:%s",
				m[1], m[2], m[3], m[5]))
			if err != nil {
				return nil, "", err
			}
			ts = ts.AddDate(0, 0, 1)
		}
		out = append(out, strings.Join([]string{
			strconv.Itoa(year), name, m[7], ts.Format("2006-01-02T15:04Z"),
		}, "\t"))
	}
	return out, dt, nil
}

// decodeEUCJP 把 EUC-JP 转成 UTF-8。
//
// 正则里要匹配「太陽」「二十四節気」「黄経」和 24 个节气名，都是真实汉字，
// 所以必须真转码。完整的 JIS X 0208 映射表有 8836 项，Go 标准库不带
// （在 golang.org/x/text 里），而本工具是 _tools 下的一次性脚本，
// 不该为它动 go.mod——那会破坏「0 新依赖」这条硬约束。
//
// 折中：调外部 iconv。macOS 与主流 Linux 都自带；缺了会明确报错而不是
// 悄悄产出乱码数据。
func decodeEUCJP(raw []byte) string {
	cmd := exec.Command("iconv", "-f", "EUC-JP", "-t", "UTF-8")
	cmd.Stdin = bytes.NewReader(raw)
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "iconv 失败：%v\n请确认系统有 iconv 命令。\n", err)
		os.Exit(1)
	}
	return string(out)
}
