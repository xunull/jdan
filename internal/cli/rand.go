package cli

import (
	crypto_rand "crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/xunull/jdan/internal/randgen"
)

// randCmdDeps 暴露外部依赖为注入点。测试时替换 randReader（fake CSPRNG）和
// exit（capture exit code）。
type randCmdDeps struct {
	out        io.Writer
	randReader io.Reader
	exit       func(code int)
}

// newRandCommand 构造 `jdan rand` 父命令及 9 个子命令。
//
// 与 jdan dns 名空间一致的二级子命令模式：父命令不接受参数，子命令分别处理
// 各 type 专属逻辑 + 共享的 --count / --json / --no-newline flag。
func newRandCommand(deps randCmdDeps) *cobra.Command {
	if deps.out == nil {
		deps.out = os.Stdout
	}
	if deps.randReader == nil {
		deps.randReader = crypto_rand.Reader
	}
	if deps.exit == nil {
		deps.exit = os.Exit
	}

	cmd := &cobra.Command{
		Use:   "rand",
		Short: "随机生成（passwords、UUIDs、hex、base64、...，CSPRNG）",
		Long: `随机生成子命令族。所有生成器使用 crypto/rand (CSPRNG)，禁止 math/rand。

字符集类生成器（password、alnum）默认排除歧义字符 I/l/1/O/0，--include-ambiguous
覆盖。所有子命令通用 --count N、--json、--no-newline 三个 flag。`,
	}

	cmd.AddCommand(
		newRandPasswordCommand(deps),
		newRandHexCommand(deps),
		newRandBase64Command(deps),
		newRandBase64URLCommand(deps),
		newRandBase32Command(deps),
		newRandAlnumCommand(deps),
		newRandUUIDCommand(deps),
		newRandIntCommand(deps),
		newRandWordCommand(deps),
	)
	return cmd
}

// ------ 共享输出 helpers ------

// addCommonFlags 在 cmd 上注册 --count / --json / --no-newline 三个共享 flag。
// 调用方传入指针。
func addCommonFlags(cmd *cobra.Command, count *int, jsonOut, noNewline *bool) {
	cmd.Flags().IntVarP(count, "count", "c", 1, "生成数量")
	cmd.Flags().BoolVarP(jsonOut, "json", "j", false, "JSON 数组输出")
	cmd.Flags().BoolVar(noNewline, "no-newline", false, "单条无换行（管道友好）；与 --count >1 互斥")
}

// validateCommonFlags 检查 count / json / no-newline 的互斥与边界。
func validateCommonFlags(count int, jsonOut, noNewline bool) error {
	if count <= 0 {
		return fmt.Errorf("--count 必须 > 0")
	}
	if jsonOut && noNewline {
		return fmt.Errorf("--json 与 --no-newline 不能同时使用")
	}
	if noNewline && count > 1 {
		return fmt.Errorf("--no-newline 仅适用 --count=1")
	}
	return nil
}

// emitStrings 是字符串生成 type 的统一输出路径。处理 N 次生成 + JSON 数组 /
// no-newline / 默认每行一条三种输出形态。
func emitStrings(deps randCmdDeps, count int, jsonOut, noNewline bool, gen func() (string, error)) error {
	if err := validateCommonFlags(count, jsonOut, noNewline); err != nil {
		return err
	}
	results := make([]string, count)
	for i := 0; i < count; i++ {
		s, err := gen()
		if err != nil {
			return err
		}
		results[i] = s
	}

	if jsonOut {
		out, err := json.MarshalIndent(results, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(deps.out, string(out))
		return nil
	}
	if noNewline {
		fmt.Fprint(deps.out, results[0])
		return nil
	}
	for _, s := range results {
		fmt.Fprintln(deps.out, s)
	}
	return nil
}

// emitInts 是 int 子命令专用——JSON 输出整数数组而非字符串数组。
func emitInts(deps randCmdDeps, count int, jsonOut bool, gen func() (int64, error)) error {
	if count <= 0 {
		return fmt.Errorf("--count 必须 > 0")
	}
	results := make([]int64, count)
	for i := 0; i < count; i++ {
		n, err := gen()
		if err != nil {
			return err
		}
		results[i] = n
	}

	if jsonOut {
		out, err := json.MarshalIndent(results, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(deps.out, string(out))
		return nil
	}
	for _, n := range results {
		fmt.Fprintln(deps.out, n)
	}
	return nil
}

// ------ password ------

func newRandPasswordCommand(deps randCmdDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "password",
		Short: "生成随机密码（默认 20 位 + 含 symbols + 排除歧义字符）",
		Long: `生成符合 1Password 风格的随机密码：默认 20 位、含特殊字符、排除歧义字符
(I/l/1/O/0)、必含每类至少一个 (lower/upper/digit/symbol)。

算法：每类先抽 1 字符放固定位置 → 剩余位置用全字符集填充 → Fisher-Yates 洗牌。
无偏差，length=4 边界也高效。

--no-symbols 退化为字母数字密码（仍要求 lower/upper/digit 每类至少一个，
与 'jdan rand alnum' 的"无类约束"明确不同）。`,
		Args: cobra.NoArgs,
	}
	var length int
	var noSymbols, includeAmbig bool
	var count int
	var jsonOut, noNewline bool

	cmd.Flags().IntVarP(&length, "length", "l", 20, "密码长度")
	cmd.Flags().BoolVar(&noSymbols, "no-symbols", false, "仅字母数字（每类仍要求至少一个）")
	cmd.Flags().BoolVar(&includeAmbig, "include-ambiguous", false, "不排除 I/l/1/O/0")
	addCommonFlags(cmd, &count, &jsonOut, &noNewline)

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return emitStrings(deps, count, jsonOut, noNewline, func() (string, error) {
			return randgen.GeneratePassword(deps.randReader, randgen.PasswordOptions{
				Length:           length,
				NoSymbols:        noSymbols,
				IncludeAmbiguous: includeAmbig,
			})
		})
	}
	return cmd
}

// ------ hex / base64 / base64url / base32 ------

// newByteEncodedCommand 通用工厂：为 hex / base64 / base64url / base32 这几个
// "字节级生成 + 编码" 的 type 复用 flag + 调用逻辑。
func newByteEncodedCommand(deps randCmdDeps, use, short, help string, defaultBytes int,
	gen func(io.Reader, int) (string, error)) *cobra.Command {
	cmd := &cobra.Command{Use: use, Short: short, Long: help, Args: cobra.NoArgs}
	var byteLen, count int
	var jsonOut, noNewline bool
	cmd.Flags().IntVarP(&byteLen, "length", "l", defaultBytes, "字节数（编码后输出更长）")
	addCommonFlags(cmd, &count, &jsonOut, &noNewline)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return emitStrings(deps, count, jsonOut, noNewline, func() (string, error) {
			return gen(deps.randReader, byteLen)
		})
	}
	return cmd
}

func newRandHexCommand(deps randCmdDeps) *cobra.Command {
	return newByteEncodedCommand(deps, "hex",
		"生成 N 字节随机的 hex 编码（默认 32 字节 → 64 hex chars）",
		"-l N 指定字节数；输出长度 = 2N。", 32, randgen.GenerateHex)
}

func newRandBase64Command(deps randCmdDeps) *cobra.Command {
	return newByteEncodedCommand(deps, "base64",
		"生成 N 字节随机的标准 base64 编码（含 + / = padding）",
		"-l N 指定字节数。", 32, randgen.GenerateBase64)
}

func newRandBase64URLCommand(deps randCmdDeps) *cobra.Command {
	return newByteEncodedCommand(deps, "base64url",
		"生成 N 字节随机的 URL-safe base64（无 + / = padding，可直接放 URL / JWT）",
		"-l N 指定字节数。", 32, randgen.GenerateBase64URL)
}

func newRandBase32Command(deps randCmdDeps) *cobra.Command {
	return newByteEncodedCommand(deps, "base32",
		"生成 N 字节随机的 RFC 4648 标准 base32（大写 A-Z + 2-7）",
		"-l N 指定字节数。Crockford 变体不支持。", 32, randgen.GenerateBase32)
}

// ------ alnum ------

func newRandAlnumCommand(deps randCmdDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "alnum",
		Short: "生成字母数字串（默认 20 位 + 排除歧义字符；与 password --no-symbols 不同：无类约束）",
		Args:  cobra.NoArgs,
	}
	var length int
	var includeAmbig bool
	var count int
	var jsonOut, noNewline bool
	cmd.Flags().IntVarP(&length, "length", "l", 20, "字符长度")
	cmd.Flags().BoolVar(&includeAmbig, "include-ambiguous", false, "不排除 I/l/1/O/0")
	addCommonFlags(cmd, &count, &jsonOut, &noNewline)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return emitStrings(deps, count, jsonOut, noNewline, func() (string, error) {
			return randgen.GenerateAlnum(deps.randReader, length, includeAmbig)
		})
	}
	return cmd
}

// ------ uuid ------

func newRandUUIDCommand(deps randCmdDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "uuid",
		Short: "生成 UUID（默认 v4 随机；-V 7 为时间排序的 v7）",
		Long: `生成 UUID。默认 v4（122 随机比特）；-V 7 切换为 v7 RFC 9562（48-bit unix-ms
时间戳 + 74-bit 随机），同毫秒内大致单调递增，适合数据库索引。

仅支持 v4 和 v7。v1 (含 MAC 地址) 和 v5 (SHA-1 命名空间) 不在 scope。`,
		Args: cobra.NoArgs,
	}
	var version int
	var count int
	var jsonOut, noNewline bool
	cmd.Flags().IntVarP(&version, "version", "V", 4, "UUID 版本（4 或 7）")
	addCommonFlags(cmd, &count, &jsonOut, &noNewline)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if version != 4 && version != 7 {
			return fmt.Errorf("UUID --version 仅支持 4 / 7，传入 %d", version)
		}
		return emitStrings(deps, count, jsonOut, noNewline, func() (string, error) {
			if version == 7 {
				return randgen.GenerateUUIDv7(deps.randReader)
			}
			return randgen.GenerateUUIDv4(deps.randReader)
		})
	}
	return cmd
}

// ------ int ------

func newRandIntCommand(deps randCmdDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "int <min> <max>",
		Short: "生成 [min, max] 闭区间内随机整数。负数请用 -- 分隔：jdan rand int -- -10 5",
		Args:  cobra.ExactArgs(2),
	}
	var count int
	var jsonOut bool
	cmd.Flags().IntVarP(&count, "count", "c", 1, "生成数量")
	cmd.Flags().BoolVarP(&jsonOut, "json", "j", false, "JSON 数组输出（输出整数数组而非字符串）")
	// int 子命令不要 --no-newline——整数 + newline 是标准 stdout 格式

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		min, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			return fmt.Errorf("min 不是合法 int64: %v", err)
		}
		max, err := strconv.ParseInt(args[1], 10, 64)
		if err != nil {
			return fmt.Errorf("max 不是合法 int64: %v", err)
		}
		return emitInts(deps, count, jsonOut, func() (int64, error) {
			return randgen.GenerateInt(deps.randReader, min, max)
		})
	}
	return cmd
}

// ------ word (diceware) ------

func newRandWordCommand(deps randCmdDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "word",
		Short: "生成 diceware 风格 passphrase（EFF Large Wordlist，7776 词）",
		Long: `从 EFF Large Wordlist (7776 词，12.9 bits 熵/词) 中均匀抽 N 词，用 sep 连接。

默认 6 词约 77 bits 熵，超过 12 字符 alnum 密码 (~71 bits)。

注意：--words 控制每个 passphrase 的词数；--count 控制 passphrase 条数。两者
不同——避免与其他子命令的 --count 语义混淆。`,
		Args: cobra.NoArgs,
	}
	var words int
	var sep string
	var count int
	var jsonOut, noNewline bool
	cmd.Flags().IntVarP(&words, "words", "w", 6, "每个 passphrase 的词数")
	cmd.Flags().String("sep", "-", "词之间的分隔符（空串合法但产生不可分割串）")
	addCommonFlags(cmd, &count, &jsonOut, &noNewline)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		sepVal, _ := cmd.Flags().GetString("sep")
		sep = sepVal
		return emitStrings(deps, count, jsonOut, noNewline, func() (string, error) {
			return randgen.GenerateWords(deps.randReader, words, sep)
		})
	}
	return cmd
}

func init() {
	rootCmd.AddCommand(newRandCommand(randCmdDeps{}))
}
