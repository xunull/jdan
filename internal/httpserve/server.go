package httpserve

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Options 控制 Run 的所有行为。零值有合理默认（root=cwd、port=0=随机、bind=0.0.0.0）。
type Options struct {
	Root         string        // 提供服务的根目录（filepath.Abs 后存）
	Port         int           // 0 = 自动找空闲端口（先试 8080）
	Bind         string        // 默认 "0.0.0.0"
	LogFormat    LogFormat     // 默认 LogText
	LogOut       io.Writer     // 默认 os.Stdout；nil = 不写日志
	Upload       bool          // 启用 POST /upload + GET /upload
	UploadDir    string        // 默认 root/uploads
	BasicAuth    string        // "user:pass" 形式；空串关闭
	Shutdown     time.Duration // Shutdown 超时；0 = 默认 5s
	OnStarted    func(addr string) // 可选：server 真正开始 listen 后回调
	RootRedirect string        // 非空时 "/" 重定向到此路径（用于单文件 serve 场景）
}

// Server 是一个已经构造好但还未跑的 server 实例。Run 阻塞直到 ctx 取消或致命错误。
// 抽出 New() / Run() 两步，方便 cli 层在 Listen 成功后拿到真实 port 打印再启动 server。
type Server struct {
	opts     Options
	addr     string // 真实 listen 的 "host:port"
	listener net.Listener
	stats    *Stats
	server   *http.Server
}

// New 解析 root 路径、找空闲端口、起 listener，但不开始 ServeHTTP。
// 调用方拿到 *Server 后可以读 Addr() 获得真实端口（用于打印 URL/QR），再调 Run。
func New(opts Options) (*Server, error) {
	if opts.Root == "" {
		opts.Root = "."
	}
	abs, err := filepath.Abs(opts.Root)
	if err != nil {
		return nil, fmt.Errorf("resolve root: %w", err)
	}
	st, err := os.Stat(abs)
	if err != nil {
		return nil, fmt.Errorf("stat root: %w", err)
	}
	if !st.IsDir() {
		return nil, fmt.Errorf("root %q is not a directory", abs)
	}
	opts.Root = abs

	if opts.Bind == "" {
		opts.Bind = "0.0.0.0"
	}
	if opts.LogOut == nil && opts.LogFormat != LogOff {
		opts.LogOut = os.Stdout
	}
	if opts.Shutdown == 0 {
		opts.Shutdown = 5 * time.Second
	}
	if opts.Upload && opts.UploadDir == "" {
		opts.UploadDir = filepath.Join(opts.Root, "uploads")
	}

	port, err := FindFreePort(opts.Port)
	if err != nil {
		return nil, err
	}
	addr := net.JoinHostPort(opts.Bind, fmt.Sprintf("%d", port))
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("listen %s: %w", addr, err)
	}

	stats := newStats()
	handler := buildHandler(opts, stats)

	s := &Server{
		opts:     opts,
		addr:     ln.Addr().String(),
		listener: ln,
		stats:    stats,
		server: &http.Server{
			Handler:      handler,
			ReadTimeout:  60 * time.Second,
			WriteTimeout: 0, // 大文件下载不能限制，留给 client 控制
			IdleTimeout:  120 * time.Second,
		},
	}
	return s, nil
}

// Addr 返回实际 bind 的 "host:port"。Port 在 Port=0 自动分配时也准确。
func (s *Server) Addr() string {
	return s.addr
}

// Port 返回实际监听端口。
func (s *Server) Port() int {
	if a, ok := s.listener.Addr().(*net.TCPAddr); ok {
		return a.Port
	}
	return 0
}

// Stats 暴露累积统计，给 cli 层在退出时打印 summary。
func (s *Server) Stats() *Stats { return s.stats }

// Run 开始服务请求，阻塞直到 ctx 取消或致命错误。返回 nil 表示优雅关闭，
// 否则返回错误。Shutdown 超时由 opts.Shutdown 控制。
func (s *Server) Run(ctx context.Context) error {
	if s.opts.OnStarted != nil {
		s.opts.OnStarted(s.addr)
	}

	errCh := make(chan error, 1)
	go func() {
		err := s.server.Serve(s.listener)
		if errors.Is(err, http.ErrServerClosed) {
			errCh <- nil
			return
		}
		errCh <- err
	}()

	select {
	case <-ctx.Done():
		shutCtx, cancel := context.WithTimeout(context.Background(), s.opts.Shutdown)
		defer cancel()
		if err := s.server.Shutdown(shutCtx); err != nil {
			return fmt.Errorf("shutdown: %w", err)
		}
		return nil
	case err := <-errCh:
		return err
	}
}

// buildHandler 装配 mux + middleware + 鉴权。
func buildHandler(opts Options, stats *Stats) http.Handler {
	mux := http.NewServeMux()

	// 上传必须先于 file server 注册，否则 "/" handler 会接管所有路径
	if opts.Upload {
		mux.HandleFunc("/upload", uploadHandler(opts.UploadDir))
		mux.HandleFunc("/upload/", uploadHandler(opts.UploadDir))
	}

	fileServer := safeFileServer(opts.Root)
	if opts.RootRedirect != "" {
		target := opts.RootRedirect
		mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/" {
				http.Redirect(w, r, target, http.StatusSeeOther)
				return
			}
			fileServer.ServeHTTP(w, r)
		}))
	} else {
		mux.Handle("/", fileServer)
	}

	var handler http.Handler = mux
	if opts.BasicAuth != "" {
		handler = withBasicAuth(opts.BasicAuth, handler)
	}
	handler = withLogging(opts.LogOut, opts.LogFormat, stats, handler)
	return handler
}

// safeFileServer 包了一层 http.FileServer，额外加 symlink-escape 防护。
// http.FileServer 自带的 cleaning 已经处理 ".." path traversal，但不防 symlink。
//
// 注意：macOS 的 /var/folders/... 实际是 /private/var/folders/... 的 symlink，
// 所以 root 本身也得用 EvalSymlinks 规范化才能跟解析后的 target 同空间比较。
func safeFileServer(root string) http.Handler {
	rootResolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		rootResolved = root
	}
	fs := http.FileServer(http.Dir(root))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clean := filepath.Clean(r.URL.Path)
		target := filepath.Join(root, clean)
		if _, err := os.Lstat(target); err == nil {
			real, err := filepath.EvalSymlinks(target)
			if err == nil {
				prefix := rootResolved + string(filepath.Separator)
				if real != rootResolved && !strings.HasPrefix(real, prefix) {
					http.NotFound(w, r)
					return
				}
			}
		}
		fs.ServeHTTP(w, r)
	})
}

// withBasicAuth 在 handler 前插一道 Basic Auth 检查。
// userPass 格式 "user:pass"；用 subtle.ConstantTimeCompare 防时序攻击。
func withBasicAuth(userPass string, next http.Handler) http.Handler {
	parts := strings.SplitN(userPass, ":", 2)
	if len(parts) != 2 {
		return next // 配置错误时不加保护比硬错误友好
	}
	wantUser, wantPass := parts[0], parts[1]
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok ||
			subtle.ConstantTimeCompare([]byte(user), []byte(wantUser)) != 1 ||
			subtle.ConstantTimeCompare([]byte(pass), []byte(wantPass)) != 1 {
			w.Header().Set("WWW-Authenticate", `Basic realm="jdan http serve"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
