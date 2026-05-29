package obsidian

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

const defaultBaseURL = "https://github.com/YishenTu/claudian/releases/latest/download"

var claudianFiles = []string{"main.js", "manifest.json", "styles.css"}

// Installer downloads and installs the Claudian Obsidian plugin.
type Installer struct {
	Client  *http.Client
	BaseURL string
}

// NewInstaller returns an Installer using the real GitHub download URL.
func NewInstaller() *Installer {
	return &Installer{
		Client:  &http.Client{},
		BaseURL: defaultBaseURL,
	}
}

// Install downloads the Claudian plugin files into vaultPath/.obsidian/plugins/claudian/.
// If the plugin directory already exists and force is false, it returns an error.
// On any download failure the partially-created directory is removed.
func (ins *Installer) Install(vaultPath string, force bool) error {
	info, err := os.Stat(vaultPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("vault 路径不存在: %s", vaultPath)
		}
		return fmt.Errorf("无法访问路径: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("不是有效目录: %s", vaultPath)
	}

	pluginDir := filepath.Join(vaultPath, ".obsidian", "plugins", "claudian")

	if _, err := os.Stat(pluginDir); err == nil && !force {
		return fmt.Errorf("Claudian 已安装在 %s，使用 --force 覆盖", pluginDir)
	}

	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		return fmt.Errorf("无法创建目录: %w", err)
	}

	fmt.Printf("正在下载 Claudian 插件到 %s\n", pluginDir)
	for _, filename := range claudianFiles {
		url := ins.BaseURL + "/" + filename
		dest := filepath.Join(pluginDir, filename)
		fmt.Printf("  - %-16s", filename)
		if err := ins.downloadFile(url, dest); err != nil {
			fmt.Println("✗")
			if rmErr := os.RemoveAll(pluginDir); rmErr != nil {
				fmt.Fprintf(os.Stderr, "警告：清理目录失败: %v\n", rmErr)
			}
			return fmt.Errorf("下载 %s 失败: %w", filename, err)
		}
		fmt.Println("✓")
	}

	fmt.Println("\n安装完成。在 Obsidian 中启用：Settings → Community plugins → Enable \"Claudian\"")
	return nil
}

func (ins *Installer) downloadFile(url, dest string) error {
	resp, err := ins.Client.Get(url)
	if err != nil {
		return fmt.Errorf("网络错误: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	f, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("创建文件失败: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		return fmt.Errorf("写入失败: %w", err)
	}

	return nil
}
