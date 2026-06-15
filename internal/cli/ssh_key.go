package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/xunull/jdan/internal/sshkey"
)

type sshKeyCmdDeps struct {
	out io.Writer
	in  io.Reader
}

func newSSHKeyCommand(deps sshKeyCmdDeps) *cobra.Command {
	if deps.out == nil {
		deps.out = os.Stdout
	}
	if deps.in == nil {
		deps.in = os.Stdin
	}
	cmd := &cobra.Command{
		Use:   "ssh-key",
		Short: "SSH 公钥/私钥工具（info / fingerprint / pubkey）",
		Long: `SSH 密钥解析工具。0 新依赖（golang.org/x/crypto/ssh）。

子命令：
  info         综合信息（类型/位数/fingerprint/comment）
  fingerprint  只出 fingerprint（SHA256 默认，--md5 切换）
  pubkey       从私钥提取公钥（= ssh-keygen -y）

输入：文件路径 / '-' stdin / 直接粘贴公钥字符串。
自动识别公钥 vs 私钥。

例：
  jdan ssh-key info ~/.ssh/id_ed25519.pub
  jdan ssh-key info ~/.ssh/id_ed25519           # 私钥
  jdan ssh-key fingerprint ~/.ssh/id_rsa.pub
  jdan ssh-key fingerprint ~/.ssh/id_rsa.pub --md5
  jdan ssh-key pubkey ~/.ssh/id_ed25519`,
	}
	cmd.AddCommand(newSSHKeyInfoCommand(deps))
	cmd.AddCommand(newSSHKeyFingerprintCommand(deps))
	cmd.AddCommand(newSSHKeyPubkeyCommand(deps))
	return cmd
}

// readKeyInput 读 key 内容：args[0] 是文件路径 / '-' stdin / 直接 key 字符串。
// 第二个返回值是源路径（用于回退读取 .pub comment），stdin/inline 时为空。
func readKeyInput(args []string, stdin io.Reader) ([]byte, string, error) {
	if len(args) == 0 || args[0] == "-" {
		data, err := io.ReadAll(stdin)
		return data, "", err
	}
	arg := args[0]
	// 直接粘贴的公钥（含算法前缀），不是文件路径
	if strings.HasPrefix(arg, "ssh-") || strings.HasPrefix(arg, "ecdsa-") || strings.HasPrefix(arg, "sk-") {
		return []byte(arg), "", nil
	}
	data, err := os.ReadFile(arg)
	if err != nil {
		return nil, "", err
	}
	return data, arg, nil
}

// ---- info ----

func newSSHKeyInfoCommand(deps sshKeyCmdDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "info [key]",
		Short:         "综合信息（吃公钥或私钥）",
		Args:          cobra.MaximumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			asJSON, _ := cmd.Flags().GetBool("json")
			passphrase, _ := cmd.Flags().GetString("passphrase")
			return runSSHKeyInfo(args, deps, asJSON, passphrase)
		},
	}
	cmd.Flags().Bool("json", false, "结构化 JSON 输出")
	cmd.Flags().String("passphrase", "", "加密私钥的口令（用于导出公钥信息）")
	return cmd
}

func runSSHKeyInfo(args []string, deps sshKeyCmdDeps, asJSON bool, passphrase string) error {
	data, srcPath, err := readKeyInput(args, deps.in)
	if err != nil {
		return err
	}
	var info sshkey.KeyInfo
	if sshkey.IsPrivateKey(data) {
		info, err = privateKeyInfo(data, srcPath, passphrase)
		if err != nil {
			return err
		}
	} else {
		pub, comment, perr := sshkey.ParsePublicKey(data)
		if perr != nil {
			return perr
		}
		info = sshkey.InfoFromPublicKey(pub, comment)
	}
	if asJSON {
		enc := json.NewEncoder(deps.out)
		enc.SetIndent("", "  ")
		return enc.Encode(info)
	}
	fmt.Fprint(deps.out, info.String())
	return nil
}

// privateKeyInfo 处理私钥：加密时只标 Encrypted（除非给了 passphrase）。
func privateKeyInfo(data []byte, srcPath, passphrase string) (sshkey.KeyInfo, error) {
	if passphrase == "" && sshkey.IsEncryptedPrivateKey(data) {
		return sshkey.KeyInfo{
			Kind:      "private",
			Type:      "OpenSSH private key",
			Encrypted: true,
		}, nil
	}
	signer, comment, err := sshkey.ParsePrivateKey(data, passphrase)
	if err != nil {
		return sshkey.KeyInfo{}, err
	}
	if comment == "" {
		comment = commentFromSiblingPub(srcPath)
	}
	return sshkey.InfoFromSigner(signer, comment), nil
}

// commentFromSiblingPub 在私钥同目录找 <name>.pub 读 comment（OpenSSH 私钥
// blob 里的 comment 不暴露给 x/crypto，所以回退读 .pub 文件）。
func commentFromSiblingPub(privPath string) string {
	if privPath == "" {
		return ""
	}
	pubPath := privPath + ".pub"
	data, err := os.ReadFile(pubPath)
	if err != nil {
		return ""
	}
	_, comment, err := sshkey.ParsePublicKey(data)
	if err != nil {
		return ""
	}
	_ = filepath.Base(pubPath)
	return comment
}

// ---- fingerprint ----

func newSSHKeyFingerprintCommand(deps sshKeyCmdDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "fingerprint [key]",
		Short:         "输出 fingerprint（SHA256 默认；--md5 切 legacy）",
		Args:          cobra.MaximumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			useMD5, _ := cmd.Flags().GetBool("md5")
			asJSON, _ := cmd.Flags().GetBool("json")
			passphrase, _ := cmd.Flags().GetString("passphrase")
			return runSSHKeyFingerprint(args, deps, useMD5, asJSON, passphrase)
		},
	}
	cmd.Flags().Bool("md5", false, "MD5 colon-hex 格式（默认 SHA256 base64）")
	cmd.Flags().Bool("json", false, "结构化 JSON 输出")
	cmd.Flags().String("passphrase", "", "加密私钥的口令")
	return cmd
}

func runSSHKeyFingerprint(args []string, deps sshKeyCmdDeps, useMD5, asJSON bool, passphrase string) error {
	data, srcPath, err := readKeyInput(args, deps.in)
	if err != nil {
		return err
	}
	var info sshkey.KeyInfo
	if sshkey.IsPrivateKey(data) {
		info, err = privateKeyInfo(data, srcPath, passphrase)
		if err != nil {
			return err
		}
		if info.Encrypted {
			return errors.New("private key is passphrase-protected; pass --passphrase to fingerprint it")
		}
	} else {
		pub, comment, perr := sshkey.ParsePublicKey(data)
		if perr != nil {
			return perr
		}
		info = sshkey.InfoFromPublicKey(pub, comment)
	}

	if asJSON {
		enc := json.NewEncoder(deps.out)
		enc.SetIndent("", "  ")
		return enc.Encode(map[string]any{
			"bits":        info.Bits,
			"fingerprint": pickFingerprint(info, useMD5),
			"comment":     info.Comment,
			"algorithm":   info.Algorithm,
		})
	}
	// 格式跟 ssh-keygen -lf 对齐：<bits> <fingerprint> <comment> (<ALGO>)
	comment := info.Comment
	if comment == "" {
		comment = "no comment"
	}
	fmt.Fprintf(deps.out, "%d %s %s (%s)\n",
		info.Bits, pickFingerprint(info, useMD5), comment, strings.ToUpper(info.Algorithm))
	return nil
}

func pickFingerprint(info sshkey.KeyInfo, useMD5 bool) string {
	if useMD5 {
		return info.FingerprintMD5
	}
	return info.FingerprintSHA
}

// ---- pubkey ----

func newSSHKeyPubkeyCommand(deps sshKeyCmdDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "pubkey [privkey]",
		Short:         "从私钥提取公钥（= ssh-keygen -y）",
		Args:          cobra.MaximumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			passphrase, _ := cmd.Flags().GetString("passphrase")
			return runSSHKeyPubkey(args, deps, passphrase)
		},
	}
	cmd.Flags().String("passphrase", "", "加密私钥的口令")
	return cmd
}

func runSSHKeyPubkey(args []string, deps sshKeyCmdDeps, passphrase string) error {
	data, srcPath, err := readKeyInput(args, deps.in)
	if err != nil {
		return err
	}
	if !sshkey.IsPrivateKey(data) {
		return errors.New("pubkey requires a private key input (got a public key)")
	}
	if passphrase == "" && sshkey.IsEncryptedPrivateKey(data) {
		return errors.New("private key is passphrase-protected; pass --passphrase")
	}
	signer, comment, err := sshkey.ParsePrivateKey(data, passphrase)
	if err != nil {
		return err
	}
	if comment == "" {
		comment = commentFromSiblingPub(srcPath)
	}
	fmt.Fprintln(deps.out, sshkey.PublicKeyLine(signer, comment))
	return nil
}

func init() {
	rootCmd.AddCommand(newSSHKeyCommand(sshKeyCmdDeps{}))
}
