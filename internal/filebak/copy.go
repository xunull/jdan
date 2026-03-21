package filebak

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// BackupFile copies src to a backup path next to src (same naming rules as design).
func BackupFile(src string, descRaw string, now time.Time) error {
	src = filepath.Clean(src)
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("源路径不是普通文件")
	}

	dst, err := BackupDestination(src, now, descRaw)
	if err != nil {
		return err
	}

	if _, err := os.Stat(dst); err == nil {
		return fmt.Errorf("已存在相同时间戳的备份")
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_EXCL, info.Mode().Perm())
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return nil
}
