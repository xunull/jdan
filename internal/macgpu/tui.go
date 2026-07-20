package macgpu

import (
	"context"
	"fmt"
	"image/color"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// GPU 功耗柱状图的满格参考值（mW）。
// Apple Silicon M 系列 GPU TDP 约 10~30W（M1 Max: ~30W，M4 Pro: ~20W）。
// 使用 30000mW 作为上界；低于该值的芯片在满载时柱状图接近满格。
const maxGPUPowerMW = 30000.0

// GPU 频率柱状图的满格参考值（MHz）。
// M4 Pro 最高约 1620MHz，取 1800 作为上界留有余量。
const maxGPUFreqMHz = 1800.0

// heatColors 是从绿到红的 100 级颜色渐变数组（color.Color 接口切片），
// 通过 lipgloss.Blend1D 生成，在包初始化时构建一次后复用。
var heatColors []color.Color

func init() {
	heatColors = lipgloss.Blend1D(100,
		lipgloss.Color("#00FF87"), // 0%  绿
		lipgloss.Color("#FFD700"), // 中段 黄
		lipgloss.Color("#FF5F5F"), // 100% 红
	)
}

// heatColor 根据 0.0~1.0 的比例返回热力颜色。
func heatColor(pct float64) color.Color {
	idx := int(pct * 99)
	if idx < 0 {
		idx = 0
	}
	if idx > 99 {
		idx = 99
	}
	return heatColors[idx]
}

// Model 是 Bubble Tea v2 的状态模型，持有 GPU 仪表盘的全部状态。
type Model struct {
	width     int
	height    int
	ready     bool
	hasDarkBG bool
	latest    *GPUSnapshot
	err       error
	interval  int
	cancel    context.CancelFunc
}

// NewModel 创建初始 TUI 模型。
func NewModel(intervalMS int, cancel context.CancelFunc) Model {
	return Model{
		interval:  intervalMS,
		cancel:    cancel,
		hasDarkBG: true, // 默认假设暗色背景；tea.BackgroundColorMsg 到来时更新
	}
}

// Init 实现 tea.Model 接口，返回初始命令。
// Collector 由 CLI 层在 program.Run 前启动，此处无需初始化 IO 命令。
func (m Model) Init() tea.Cmd {
	return nil
}

// Update 实现 tea.Model 接口，处理所有消息。
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true

	case tea.BackgroundColorMsg:
		m.hasDarkBG = msg.IsDark()

	case SampleMsg:
		m.latest = msg.Snapshot
		m.err = nil

	case ErrMsg:
		m.err = msg.Err

	case tea.KeyPressMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.cancel()
			return m, tea.Quit
		}
	}
	return m, nil
}

// View 实现 tea.Model 接口，返回当前帧的完整渲染结果。
// AltScreen = true 让程序占用全屏终端缓冲区（类 htop 效果）。
func (m Model) View() tea.View {
	if !m.ready {
		v := tea.NewView("Initializing...\n\nPress q to quit.")
		v.AltScreen = true
		return v
	}
	v := tea.NewView(m.render())
	v.AltScreen = true
	return v
}

func (m Model) render() string {
	var b strings.Builder

	lightDark := lipgloss.LightDark(m.hasDarkBG)

	titleColor := lightDark(lipgloss.Color("#1a1a2e"), lipgloss.Color("#e0e0ff"))
	labelColor := lightDark(lipgloss.Color("#555555"), lipgloss.Color("#bbbbbb"))
	dimColor := lightDark(lipgloss.Color("#888888"), lipgloss.Color("#555555"))

	// 标题栏
	now := time.Now().Format("15:04:05")
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(titleColor).
		Render(fmt.Sprintf("jdan macgpu  interval: %dms  %s", m.interval, now))
	b.WriteString(title)
	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().Foreground(dimColor).Render(strings.Repeat("─", clamp(m.width, 20, 60))))
	b.WriteString("\n\n")

	// 柱状图区域
	barWidth := m.width - 28
	barWidth = clamp(barWidth, 10, 50)

	if m.latest != nil {
		freqPct := m.latest.FreqMHz / maxGPUFreqMHz
		powerPct := m.latest.PowerMW / maxGPUPowerMW

		b.WriteString(renderBar("GPU 使用率", m.latest.ActiveResidency, barWidth, labelColor))
		b.WriteString("\n")
		b.WriteString(renderBar("GPU 功耗  ", powerPct, barWidth, labelColor))
		b.WriteString("\n")
		b.WriteString(renderBar("GPU 频率  ", freqPct, barWidth, labelColor))
		b.WriteString("\n\n")

		// 详情表格
		b.WriteString(lipgloss.NewStyle().Foreground(dimColor).Render(strings.Repeat("─", clamp(m.width, 20, 60))))
		b.WriteString("\n")
		b.WriteString(renderDetailTable(m.latest, labelColor, dimColor))
	} else if m.err == nil {
		b.WriteString(lipgloss.NewStyle().Foreground(dimColor).Render("等待第一次采样..."))
		b.WriteString("\n")
	}

	// 错误提示
	if m.err != nil {
		errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5F5F")).Bold(true)
		b.WriteString(errStyle.Render(fmt.Sprintf("错误: %v", m.err)))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().Foreground(dimColor).Render("按 q 退出"))

	return b.String()
}

// renderBar 渲染一条带颜色的 ASCII 水平柱状图。
// label: 左侧标签（固定宽度）
// pct: 0.0 ~ 1.0 的比例值
// width: 柱子最大字符宽度
// labelColor: 标签颜色
func renderBar(label string, pct float64, width int, labelColor color.Color) string {
	pct = clampF(pct, 0, 1)

	filled := int(pct * float64(width))
	empty := width - filled

	c := heatColor(pct)

	bar := lipgloss.NewStyle().Foreground(c).Render(strings.Repeat("█", filled))
	emptyBar := lipgloss.NewStyle().Foreground(lipgloss.Color("#333333")).Render(strings.Repeat("░", empty))

	labelStyle := lipgloss.NewStyle().Width(10).Foreground(labelColor)
	pctStyle := lipgloss.NewStyle().Width(7).Foreground(c).Bold(true)

	return fmt.Sprintf("%s [%s%s] %s",
		labelStyle.Render(label),
		bar,
		emptyBar,
		pctStyle.Render(fmt.Sprintf("%5.1f%%", pct*100)),
	)
}

// renderDetailTable 渲染底部详情表格。
func renderDetailTable(s *GPUSnapshot, labelColor, dimColor color.Color) string {
	lStyle := lipgloss.NewStyle().Width(12).Foreground(labelColor)
	vStyle := lipgloss.NewStyle().Bold(true)

	thermal := s.ThermalPressure
	if thermal == "" {
		thermal = "N/A"
	}
	thermalColor := thermalPressureColor(thermal)

	ts := "N/A"
	if !s.SampledAt.IsZero() {
		ts = s.SampledAt.Format("15:04:05")
	}

	rows := []string{
		fmt.Sprintf("%s %s",
			lStyle.Render("GPU 使用率"),
			vStyle.Foreground(heatColor(s.ActiveResidency)).Render(fmt.Sprintf("%.1f%%", s.ActiveResidency*100))),
		fmt.Sprintf("%s %s",
			lStyle.Render("GPU 频率"),
			vStyle.Render(fmt.Sprintf("%.0f MHz", s.FreqMHz))),
		fmt.Sprintf("%s %s",
			lStyle.Render("GPU 功耗"),
			vStyle.Render(fmt.Sprintf("%.1f mW", s.PowerMW))),
		fmt.Sprintf("%s %s",
			lStyle.Render("散热压力"),
			vStyle.Foreground(thermalColor).Render(thermal)),
		fmt.Sprintf("%s %s",
			lStyle.Render("采样时间"),
			lipgloss.NewStyle().Foreground(dimColor).Render(ts)),
	}

	return strings.Join(rows, "\n") + "\n"
}

// thermalPressureColor 根据散热压力等级返回对应颜色。
func thermalPressureColor(level string) color.Color {
	switch level {
	case "Nominal":
		return lipgloss.Color("#00FF87")
	case "Light":
		return lipgloss.Color("#ADFF2F")
	case "Moderate":
		return lipgloss.Color("#FFD700")
	case "Heavy":
		return lipgloss.Color("#FF8C00")
	case "Critical":
		return lipgloss.Color("#FF5F5F")
	default:
		return lipgloss.Color("#888888")
	}
}

// clamp 将整数限制在 [lo, hi] 范围内。
func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// clampF 将浮点数限制在 [lo, hi] 范围内。
func clampF(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
