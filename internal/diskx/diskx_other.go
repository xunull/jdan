//go:build !darwin && !linux

package diskx

import "errors"

var errUnsupported = errors.New("jdan disk 暂不支持该平台（仅 darwin / linux）")

// Mounts 在非 darwin/linux 平台返回不支持错误。
func Mounts() ([]Mount, error) { return nil, errUnsupported }

// StatPath 在非 darwin/linux 平台返回不支持错误。
func StatPath(string) (Mount, error) { return Mount{}, errUnsupported }
