// Package fake 实现 jdan fake 命令的核心：生成像真实数据的结构化假值
// （姓名 / 邮箱 / UUID / 句子 / 整数 / 日期 / IP），供造测试 fixture、填库、写示例。
//
// 全部值取自内置示例词库或 RFC 保留段，不对应真实个人/主机。默认从 crypto/rand
// 取熵 → 每次不同；给定 seed 则用 math/rand 确定性序列 → 同 seed 同输出，便于
// 造稳定 fixture。
package fake

import (
	cryptorand "crypto/rand"
	"encoding/binary"
	"fmt"
	"math/rand"
	"strings"
	"time"
)

// SupportedTypes 是 jdan fake <type> 支持的标量类型（排序）。
var SupportedTypes = []string{"date", "email", "int", "ip", "name", "sentence", "uuid", "word"}

// Generator 基于一个 *rand.Rand 生成假数据。非线程安全。
type Generator struct {
	rng *rand.Rand
}

// New 用确定性种子构造 Generator：同 seed → 同输出序列。
func New(seed int64) *Generator {
	return &Generator{rng: rand.New(rand.NewSource(seed))}
}

// NewRandom 从 crypto/rand 取熵作为种子，每次不同。
func NewRandom() (*Generator, error) {
	var b [8]byte
	if _, err := cryptorand.Read(b[:]); err != nil {
		return nil, err
	}
	seed := int64(binary.LittleEndian.Uint64(b[:]))
	return &Generator{rng: rand.New(rand.NewSource(seed))}, nil
}

func (g *Generator) pick(list []string) string {
	return list[g.rng.Intn(len(list))]
}

// FirstName 返回一个名。
func (g *Generator) FirstName() string { return g.pick(firstNames) }

// LastName 返回一个姓。
func (g *Generator) LastName() string { return g.pick(lastNames) }

// Name 返回「名 姓」。
func (g *Generator) Name() string {
	return g.FirstName() + " " + g.LastName()
}

// emailFrom 由姓名派生邮箱：first.last@domain（小写）。
func (g *Generator) emailFrom(first, last string) string {
	local := strings.ToLower(first) + "." + strings.ToLower(last)
	return local + "@" + g.pick(domains)
}

// Email 返回一个随机邮箱。
func (g *Generator) Email() string {
	return g.emailFrom(g.FirstName(), g.LastName())
}

// Word 返回一个 lorem 词。
func (g *Generator) Word() string { return g.pick(loremWords) }

// Sentence 返回 n 个词组成的句子（首字母大写、句末加点）。n<=0 取 1。
func (g *Generator) Sentence(n int) string {
	if n <= 0 {
		n = 1
	}
	words := make([]string, n)
	for i := range words {
		words[i] = g.Word()
	}
	s := strings.Join(words, " ")
	return strings.ToUpper(s[:1]) + s[1:] + "."
}

// Int 返回 [min, max] 闭区间内的随机整数。若 min>max 则交换。
func (g *Generator) Int(min, max int) int {
	if min > max {
		min, max = max, min
	}
	return min + g.rng.Intn(max-min+1)
}

// dateRangeStart / dateRangeDays 定义一个固定窗口 [2000-01-01, 2025-01-01)，
// 不依赖 wall clock，保证 --seed 可复现。
var dateRangeStart = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)

const dateRangeDays = 9131 // 2000-01-01 .. 2024-12-31

// Date 返回固定窗口内的随机日期，按 format 格式化（Go 参考时间布局）。
func (g *Generator) Date(format string) string {
	if format == "" {
		format = "2006-01-02"
	}
	d := dateRangeStart.AddDate(0, 0, g.rng.Intn(dateRangeDays))
	return d.Format(format)
}

// IP 返回一个 RFC 5737 文档段内的随机 IPv4（不可路由，安全）。
func (g *Generator) IP() string {
	return fmt.Sprintf("%s.%d", g.pick(docIPBlocks), g.rng.Intn(256))
}

// UUID 返回一个 v4 格式 UUID（用本 Generator 的 rng，故 --seed 也可复现）。
func (g *Generator) UUID() string {
	var b [16]byte
	for i := range b {
		b[i] = byte(g.rng.Intn(256))
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// Value 按 type 返回一个标量值的字符串形式。opts 提供 int/sentence/date 的参数。
func (g *Generator) Value(typ string, opts Options) (string, error) {
	switch typ {
	case "name":
		return g.Name(), nil
	case "email":
		return g.Email(), nil
	case "uuid":
		return g.UUID(), nil
	case "word":
		return g.Word(), nil
	case "sentence":
		return g.Sentence(opts.Words), nil
	case "int":
		return fmt.Sprintf("%d", g.Int(opts.Min, opts.Max)), nil
	case "date":
		return g.Date(opts.DateFormat), nil
	case "ip":
		return g.IP(), nil
	default:
		return "", fmt.Errorf("unknown type %q (available: %s)", typ, strings.Join(SupportedTypes, ", "))
	}
}

// Options 携带类型相关参数。
type Options struct {
	Min        int
	Max        int
	Words      int
	DateFormat string
}

// Person 是复合记录（jdan fake --json 无 type 时）。
type Person struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Age   int    `json:"age"`
	IP    string `json:"ip"`
}

// Person 生成一条复合记录，email 与 name 一致。
func (g *Generator) Person() Person {
	first, last := g.FirstName(), g.LastName()
	return Person{
		Name:  first + " " + last,
		Email: g.emailFrom(first, last),
		Age:   g.Int(18, 80),
		IP:    g.IP(),
	}
}
