package randgen

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
)

// EFF Large Wordlist 嵌入二进制。来源：
//
//	https://www.eff.org/files/2016/07/18/eff_large_wordlist.txt
//
// 许可：Creative Commons CC-BY 3.0（可分发，需署名）。
// 词数：7776 个；熵：12.9 bits / 词（log2(7776)）。
// 格式：每行 `NNNNN<TAB>word`，N 是 5 位 dice roll prefix。我们丢弃 prefix
// 只保留 word。
//
//go:embed diceware_words.txt
var dicewareRaw []byte

// effLargeSHA256 是 EFF Large List 的官方 SHA256。
// 来源：EFF 2016-07-18 发布，文件未变。
// 若有人篡改嵌入的 word list，init() 会 panic——这是 supply chain 安全门禁。
const effLargeSHA256 = "addd35536511597a02fa0a9ff1e5284677b8883b83e986e43f15a3db996b903e"

// expectedWordCount 是 EFF Large List 的词数。
const expectedWordCount = 7776

// dicewareWords 是解析后的 7776 词列表，init() 时填充。
var dicewareWords []string

func init() {
	// 1. 校验内嵌内容的 SHA256 与官方一致
	sum := sha256.Sum256(dicewareRaw)
	gotSHA := hex.EncodeToString(sum[:])
	if gotSHA != effLargeSHA256 {
		panic(fmt.Sprintf(
			"diceware word list SHA256 mismatch (supply chain check failed):\n"+
				"  got:  %s\n"+
				"  want: %s",
			gotSHA, effLargeSHA256))
	}

	// 2. 解析 NNNNN<TAB>word 格式
	scanner := bufio.NewScanner(bytes.NewReader(dicewareRaw))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		// 找到第一个 tab 或空格——dice prefix 与 word 的分隔符
		idx := strings.IndexAny(line, "\t ")
		var word string
		if idx < 0 {
			word = line
		} else {
			word = strings.TrimSpace(line[idx+1:])
		}
		if word != "" {
			dicewareWords = append(dicewareWords, word)
		}
	}

	// 3. 验证词数
	if len(dicewareWords) != expectedWordCount {
		panic(fmt.Sprintf(
			"diceware word list parse error: expected %d words, got %d",
			expectedWordCount, len(dicewareWords)))
	}
}

// GenerateWords 从 EFF Large Wordlist 中均匀抽 count 个词，用 sep 连接。
//
// sep 可以是任意字符串，含空串（空串 + count > 1 会得到不可分割的连接串，
// 这是用户的选择，不是错误）。
func GenerateWords(reader io.Reader, count int, sep string) (string, error) {
	if count <= 0 {
		return "", errors.New("word count must be > 0")
	}
	parts := make([]string, count)
	for i := 0; i < count; i++ {
		idx, err := randIndex(reader, len(dicewareWords))
		if err != nil {
			return "", err
		}
		parts[i] = dicewareWords[idx]
	}
	return strings.Join(parts, sep), nil
}

// DicewareCount 返回内嵌 word list 的词数。供测试断言和 README 引用。
func DicewareCount() int {
	return len(dicewareWords)
}
