package tradx

import (
	"bufio"
	"bytes"
	"embed"
	"fmt"
	"strings"
	"sync"
	"unicode/utf8"
)

//go:embed data/*.txt
var dataFS embed.FS

// dict 是一部词典：key→默认值（多值取第一个），外加本词典最长 key 的 rune 数。
type dict struct {
	m   map[string]string
	max int
}

// matchPrefix 返回本词典在 rs[i:] 处的**最长**命中（对齐 OpenCC Dict::MatchPrefix）。
func (d *dict) matchPrefix(rs []rune, i int) (n int, val string, ok bool) {
	hi := d.max
	if rem := len(rs) - i; rem < hi {
		hi = rem
	}
	for L := hi; L >= 1; L-- {
		if v, ok := d.m[string(rs[i:i+L])]; ok {
			return L, v, true
		}
	}
	return 0, "", false
}

// parseDict 解析 OpenCC 词典文本。格式 `key\tv1 v2 …`（空格分隔多值，取第一个）。
// 按内容跳过空行与 # 注释行（不硬编码跳固定行数，防上游改注释头时吞真数据）。
func parseDict(data []byte) (*dict, error) {
	d := &dict{m: make(map[string]string)}
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		tab := strings.IndexByte(line, '\t')
		if tab <= 0 {
			continue // 无 key 或无制表符，跳过
		}
		key := line[:tab]
		val := line[tab+1:]
		if sp := strings.IndexByte(val, ' '); sp >= 0 {
			val = val[:sp] // 多值取第一个（OpenCC 默认）
		}
		if val == "" {
			continue
		}
		d.m[key] = val
		if n := utf8.RuneCountInString(key); n > d.max {
			d.max = n
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return d, nil
}

// 懒加载：~1.18MB 词典只在首次转换时解析一次，不拖累其余 jdan 子命令启动。
var (
	loadOnce sync.Once
	dicts    map[string]*dict
	loadErr  error
)

var dictFiles = []string{
	"STCharacters", "STPhrases",
	"TSCharacters", "TSPhrases",
	"TWPhrases", "TWVariants", "TWVariantsPhrases",
	"HKVariants", "HKVariantsPhrases",
}

func loadDicts() {
	dicts = make(map[string]*dict, len(dictFiles))
	for _, name := range dictFiles {
		b, err := dataFS.ReadFile("data/" + name + ".txt")
		if err != nil {
			loadErr = fmt.Errorf("读取内嵌词典 %s: %w", name, err)
			return
		}
		d, err := parseDict(b)
		if err != nil {
			loadErr = fmt.Errorf("解析内嵌词典 %s: %w", name, err)
			return
		}
		if len(d.m) == 0 {
			loadErr = fmt.Errorf("内嵌词典 %s 解析后为空", name)
			return
		}
		dicts[name] = d
	}
}

// getDicts 触发懒加载并返回词典表；解析失败会显式报错（不吞成空表）。
func getDicts() (map[string]*dict, error) {
	loadOnce.Do(loadDicts)
	if loadErr != nil {
		return nil, loadErr
	}
	return dicts, nil
}
