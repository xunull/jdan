// Package morsex 文本 ↔ 国际摩斯电码（ITU）互转。字母间单空格、单词间 " / "。
// 大小写无关，解码输出大写。纯查表，无外部依赖。
package morsex

import "strings"

// forward 是 字符 → 摩斯码 的映射（ITU 国际码）。
var forward = map[rune]string{
	'A': ".-", 'B': "-...", 'C': "-.-.", 'D': "-..", 'E': ".",
	'F': "..-.", 'G': "--.", 'H': "....", 'I': "..", 'J': ".---",
	'K': "-.-", 'L': ".-..", 'M': "--", 'N': "-.", 'O': "---",
	'P': ".--.", 'Q': "--.-", 'R': ".-.", 'S': "...", 'T': "-",
	'U': "..-", 'V': "...-", 'W': ".--", 'X': "-..-", 'Y': "-.--",
	'Z': "--..",
	'0': "-----", '1': ".----", '2': "..---", '3': "...--", '4': "....-",
	'5': ".....", '6': "-....", '7': "--...", '8': "---..", '9': "----.",
	'.': ".-.-.-", ',': "--..--", '?': "..--..", '\'': ".----.", '!': "-.-.--",
	'/': "-..-.", '(': "-.--.", ')': "-.--.-", '&': ".-...", ':': "---...",
	';': "-.-.-.", '=': "-...-", '+': ".-.-.", '-': "-....-", '_': "..--.-",
	'"': ".-..-.", '$': "...-..-", '@': ".--.-.",
}

// reverse 由 forward 反推（摩斯码 → 字符）。
var reverse = func() map[string]rune {
	m := make(map[string]rune, len(forward))
	for r, c := range forward {
		m[c] = r
	}
	return m
}()

// LooksLikeMorse 判断输入是否「只含摩斯字符」（用于自动判方向）。
func LooksLikeMorse(s string) bool {
	t := strings.TrimSpace(s)
	if t == "" {
		return false
	}
	for _, r := range t {
		switch r {
		case '.', '-', '/', ' ', '\t', '\n', '\r':
		default:
			return false
		}
	}
	return true
}

// Encode 把文本编码成摩斯码。返回 (摩斯码, 跳过的无法编码字符数)。
func Encode(s string) (string, int) {
	skipped := 0
	var words []string
	for w := range strings.FieldsSeq(strings.ToUpper(s)) {
		var codes []string
		for _, r := range w {
			if code, ok := forward[r]; ok {
				codes = append(codes, code)
			} else {
				skipped++
			}
		}
		if len(codes) > 0 {
			words = append(words, strings.Join(codes, " "))
		}
	}
	return strings.Join(words, " / "), skipped
}

// Decode 把摩斯码解码成大写文本。单词以 "/" 分隔。返回 (文本, 无法识别的码数)。
func Decode(s string) (string, int) {
	unknown := 0
	var words []string
	for chunk := range strings.SplitSeq(s, "/") {
		chunk = strings.TrimSpace(chunk)
		if chunk == "" {
			continue
		}
		var letters []rune
		for code := range strings.FieldsSeq(chunk) {
			if r, ok := reverse[code]; ok {
				letters = append(letters, r)
			} else {
				letters = append(letters, '#')
				unknown++
			}
		}
		words = append(words, string(letters))
	}
	return strings.Join(words, " "), unknown
}
