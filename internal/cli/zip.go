package cli

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var zipCmd = &cobra.Command{
	Use:   "zip [path]",
	Short: "使用 zip 压缩文件或目录",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		src := args[0]

		// 检查文件/目录是否存在
		info, err := os.Stat(src)
		if os.IsNotExist(err) {
			return err
		}

		// 获取文件名（不含路径）
		baseName := filepath.Base(src)

		// 输出文件名 = 原文件名 + .zip
		zipName := baseName + ".zip"

		// 创建 zip 文件
		zipFile, err := os.Create(zipName)
		if err != nil {
			return err
		}
		defer zipFile.Close()

		// 创建 zip writer
		zipWriter := zip.NewWriter(zipFile)
		defer zipWriter.Close()

		// 如果是目录，递归添加
		if info.IsDir() {
			err = addDirToZip(zipWriter, src, baseName)
		} else {
			err = addFileToZip(zipWriter, src, baseName)
		}

		if err != nil {
			return err
		}

		fmt.Printf("已创建压缩文件: %s\n", zipName)
		return nil
	},
}

func addFileToZip(w *zip.Writer, filePath, nameInZip string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	header := &zip.FileHeader{
		Name:   nameInZip,
		Method: zip.Deflate,
	}

	writer, err := w.CreateHeader(header)
	if err != nil {
		return err
	}

	_, err = io.Copy(writer, file)
	return err
}

func addDirToZip(w *zip.Writer, dirPath, baseName string) error {
	return filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 跳过根目录自身
		if path == dirPath {
			return nil
		}

		// 计算相对于 dirPath 的路径
		relPath, err := filepath.Rel(dirPath, path)
		if err != nil {
			return err
		}

		// 在 zip 中使用 baseName 作为根目录
		nameInZip := filepath.Join(baseName, relPath)

		if info.IsDir() {
			// 目录不需要添加，实际文件会处理
			return nil
		}

		return addFileToZip(w, path, nameInZip)
	})
}

func init() {
	rootCmd.AddCommand(zipCmd)
}
