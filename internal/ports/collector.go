package ports

import (
	"bytes"
	"fmt"
	"os/exec"
	"regexp"
	"sort"
	"strings"
)

type PortEntry struct {
	Protocol string `json:"protocol"`
	Address  string `json:"address"`
	Port     int    `json:"port"`
	Process  string `json:"process"`
}

var nameRegex = regexp.MustCompile(`^(.+):(\d+)(?:\s+\((.+)\))?$`)

func CollectPorts(tcp, udp bool) ([]PortEntry, error) {
	var entries []PortEntry

	if tcp {
		items, err := collectTCP()
		if err != nil {
			return nil, err
		}
		entries = append(entries, items...)
	}

	if udp {
		items, err := collectUDP()
		if err != nil {
			return nil, err
		}
		entries = append(entries, items...)
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Port != entries[j].Port {
			return entries[i].Port < entries[j].Port
		}
		return entries[i].Protocol < entries[j].Protocol
	})

	return entries, nil
}

func collectTCP() ([]PortEntry, error) {
	args := []string{"-iTCP", "-P", "-n", "-sTCP:LISTEN"}
	cmd := exec.Command("lsof", args...)
	out, err := cmd.Output()
	if err != nil {
		if strings.Contains(err.Error(), "executable file not found") {
			return nil, fmt.Errorf("lsof 命令不存在，请确保已安装 lsof")
		}
		return nil, fmt.Errorf("lsof 执行失败: %w", err)
	}

	return parseLines(string(out), "TCP")
}

func collectUDP() ([]PortEntry, error) {
	args := []string{"-iUDP", "-P", "-n"}
	cmd := exec.Command("lsof", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	out, err := cmd.Output()
	if err != nil {
		if strings.Contains(err.Error(), "executable file not found") {
			return nil, fmt.Errorf("lsof 命令不存在，请确保已安装 lsof")
		}
		return nil, fmt.Errorf("lsof 执行失败: %w", err)
	}

	entries, err := parseLines(string(out), "UDP")
	if err != nil {
		return nil, err
	}

	var filtered []PortEntry
	for _, e := range entries {
		if strings.Contains(e.Address, "->") {
			continue
		}
		filtered = append(filtered, e)
	}
	return filtered, nil
}

func parseLines(output, protocol string) ([]PortEntry, error) {
	lines := strings.Split(output, "\n")
	var entries []PortEntry

	for _, line := range lines[1:] {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 9 {
			continue
		}

		process := fields[0]
		nameField := fields[8]

		matches := nameRegex.FindStringSubmatch(nameField)
		if matches == nil {
			continue
		}

		addr := matches[1]
		portStr := matches[2]

		var port int
		if _, err := fmt.Sscanf(portStr, "%d", &port); err != nil {
			continue
		}

		entries = append(entries, PortEntry{
			Protocol: protocol,
			Address:  addr,
			Port:     port,
			Process:  process,
		})
	}

	return entries, nil
}
