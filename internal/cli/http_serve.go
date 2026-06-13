package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/xunull/jdan/internal/httpserve"
	"github.com/xunull/jdan/internal/qrcode"
)

type httpServeCmdDeps struct {
	out io.Writer
}

func newHTTPServeCommand(deps httpServeCmdDeps) *cobra.Command {
	if deps.out == nil {
		deps.out = os.Stdout
	}
	cmd := &cobra.Command{
		Use:   "serve [path]",
		Short: "临时静态文件服务器 + LAN URL + 终端二维码",
		Long: `临时启一个静态文件服务器。启动时自动：
  - 检测局域网 IP（RFC1918 私有段）
  - 在终端打印 LAN URL 的二维码（复用 jdan qr）
  - 监听访问日志，Ctrl+C 优雅关闭

例：
  jdan http serve                       # 服务当前目录
  jdan http serve ~/Downloads
  jdan http serve report.pdf            # 单文件 → / 重定向到 /report.pdf
  jdan http serve . --port 9000
  jdan http serve . --bind 127.0.0.1    # 仅 localhost
  jdan http serve . --upload            # POST /upload + GET /upload 表单
  jdan http serve . --auth alice:secret # Basic Auth
  jdan http serve . --no-qr --quiet     # 不打印二维码、不打访问日志`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runHTTPServe(cmd, args, deps.out)
		},
	}
	cmd.Flags().Int("port", 0, "端口（0 = 自动找空闲端口，从 8080 开始）")
	cmd.Flags().String("bind", "0.0.0.0", "绑定地址")
	cmd.Flags().Bool("no-qr", false, "不打印终端二维码")
	cmd.Flags().Bool("upload", false, "启用 POST /upload + GET /upload 表单")
	cmd.Flags().String("upload-dir", "", "上传目录（默认 <root>/uploads）")
	cmd.Flags().String("auth", "", "Basic Auth user:pass")
	cmd.Flags().Bool("quiet", false, "不打访问日志")
	cmd.Flags().Bool("json", false, "访问日志输出 ndjson")
	return cmd
}

func runHTTPServe(cmd *cobra.Command, args []string, out io.Writer) error {
	port, _ := cmd.Flags().GetInt("port")
	bind, _ := cmd.Flags().GetString("bind")
	noQR, _ := cmd.Flags().GetBool("no-qr")
	upload, _ := cmd.Flags().GetBool("upload")
	uploadDir, _ := cmd.Flags().GetString("upload-dir")
	auth, _ := cmd.Flags().GetString("auth")
	quiet, _ := cmd.Flags().GetBool("quiet")
	asJSON, _ := cmd.Flags().GetBool("json")

	rawPath := "."
	if len(args) > 0 {
		rawPath = args[0]
	}
	root, redirect, err := resolveServePath(rawPath)
	if err != nil {
		return err
	}

	logFormat := httpserve.LogText
	if quiet {
		logFormat = httpserve.LogOff
	} else if asJSON {
		logFormat = httpserve.LogJSON
	}

	opts := httpserve.Options{
		Root:         root,
		Port:         port,
		Bind:         bind,
		LogFormat:    logFormat,
		LogOut:       out,
		Upload:       upload,
		UploadDir:    uploadDir,
		BasicAuth:    auth,
		RootRedirect: redirect,
	}

	srv, err := httpserve.New(opts)
	if err != nil {
		return err
	}

	printStartBanner(out, root, bind, srv.Port(), redirect, upload, noQR)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	runErr := srv.Run(ctx)

	reqs, bytesN, clients := srv.Stats().Snapshot()
	fmt.Fprintf(out, "\nserved %d request(s) to %d client(s), %s total\n",
		reqs, clients, humanBytes(bytesN))

	if runErr == nil || errors.Is(runErr, http.ErrServerClosed) {
		return nil
	}
	return runErr
}

// resolveServePath 把用户传的 path 解析成 (root, redirect)：
//   - path 是目录 → root = path 的绝对路径，redirect 为空
//   - path 是文件 → root = 父目录绝对路径，redirect = "/<basename>"
func resolveServePath(p string) (root, redirect string, err error) {
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", "", err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", "", err
	}
	if info.IsDir() {
		return abs, "", nil
	}
	return filepath.Dir(abs), "/" + filepath.Base(abs), nil
}

func printStartBanner(out io.Writer, root, bind string, port int, redirect string, upload, noQR bool) {
	if bind == "0.0.0.0" || bind == "::" {
		fmt.Fprintf(out, "⚠  serving on all interfaces (%s:%d) — anyone on your LAN can read these files\n", bind, port)
		fmt.Fprintf(out, "   to limit to localhost: --bind 127.0.0.1\n\n")
	}

	fmt.Fprintf(out, "serving %s on:\n", root)
	fmt.Fprintf(out, "  http://localhost:%d%s\n", port, redirect)

	lanIPs, _ := httpserve.DetectLANIPs()
	var primaryURL string
	for _, ip := range lanIPs {
		url := fmt.Sprintf("http://%s%s", net.JoinHostPort(ip.String(), fmt.Sprintf("%d", port)), redirect)
		fmt.Fprintf(out, "  %s\n", url)
		if primaryURL == "" {
			primaryURL = url
		}
	}
	if upload {
		fmt.Fprintf(out, "  (upload form available at /upload)\n")
	}
	fmt.Fprintln(out)

	if !noQR && primaryURL != "" {
		s, err := qrcode.RenderTerminal(primaryURL, qrcode.Options{ECC: qrcode.Medium})
		if err == nil {
			fmt.Fprint(out, s)
		}
	}
	fmt.Fprintln(out, "press Ctrl+C to stop")
	fmt.Fprintln(out)
}

func humanBytes(b uint64) string {
	const k = 1024
	if b == 0 {
		return "0B"
	}
	if b < k {
		return fmt.Sprintf("%dB", b)
	}
	if b < k*k {
		return fmt.Sprintf("%.1fKB", float64(b)/k)
	}
	if b < k*k*k {
		return fmt.Sprintf("%.1fMB", float64(b)/(k*k))
	}
	return fmt.Sprintf("%.1fGB", float64(b)/(k*k*k))
}

func init() {
	httpCmd.AddCommand(newHTTPServeCommand(httpServeCmdDeps{}))
}
