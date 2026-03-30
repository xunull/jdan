package macgpu

import (
	"bytes"
	"time"

	"howett.net/plist"
)

// PowerMetricsSample 对应 powermetrics --format plist 输出的顶层结构。
// 字段缺失时返回零值，兼容不同芯片和 macOS 版本。
type PowerMetricsSample struct {
	ElapsedNS       uint64         `plist:"elapsed_ns"`
	HWModel         string         `plist:"hw_model"`
	Timestamp       time.Time      `plist:"timestamp"` // plist date 类型，直接映射到 time.Time
	Processor       ProcessorStats `plist:"processor"`
	ThermalPressure string         `plist:"thermal_pressure"`
	GPU             GPUStats       `plist:"gpu"`
}

// ProcessorStats 包含 CPU/GPU 能耗汇总。
type ProcessorStats struct {
	GPUPowerMW    float64 `plist:"gpu_power"`
	GPUEnergyMJ   int64   `plist:"gpu_energy"`
	CPUPowerMW    float64 `plist:"cpu_power"`
	ANEPowerMW    float64 `plist:"ane_power"`
	CombinedPower float64 `plist:"combined_power"`
}

// GPUStats 包含 GPU 的频率、空闲占比和频率档位。
//
// 注意：plist 中 freq_hz 字段名具有误导性——
// GPU 的该字段单位实际是 MHz（非 Hz），与 CPU cluster 同名字段行为不同。
// 这是 Apple powermetrics 的已知命名不一致。
type GPUStats struct {
	FreqMHz    float64     `plist:"freq_hz"`
	IdleRatio  float64     `plist:"idle_ratio"`
	IdleNS     int64       `plist:"idle_ns"`
	DVFMStates []DVFMState `plist:"dvfm_states"`
	EnergyMJ   int64       `plist:"gpu_energy"`
}

// ActiveResidency 返回 GPU 活跃占用率（0.0 ~ 1.0）。
// powermetrics 不直接提供活跃占比字段，需由 1 - idle_ratio 计算。
func (g *GPUStats) ActiveResidency() float64 {
	active := 1.0 - g.IdleRatio
	if active < 0 {
		return 0
	}
	if active > 1 {
		return 1
	}
	return active
}

// DVFMState 表示 GPU 在某个频率档位的时间分布。
type DVFMState struct {
	FreqMHz   int     `plist:"freq"`
	UsedRatio float64 `plist:"used_ratio"`
	UsedNS    int64   `plist:"used_ns"`
}

// GPUSnapshot 是供 TUI 层消费的扁平化 GPU 指标快照，
// 避免 TUI 直接依赖 plist 结构细节。
type GPUSnapshot struct {
	// ActiveResidency 是 GPU 活跃占用率，范围 0.0 ~ 1.0。
	ActiveResidency float64
	// FreqMHz 是 GPU 加权平均活跃频率，单位 MHz。
	FreqMHz float64
	// PowerMW 是 GPU 平均功耗，单位 mW。
	PowerMW float64
	// ThermalPressure 是散热压力等级字符串：
	// "Nominal" | "Light" | "Moderate" | "Heavy" | "Critical"
	ThermalPressure string
	// SampledAt 是采样时间戳（解析自 plist timestamp 字段，失败则为零值）。
	SampledAt time.Time
}

// ParseSample 将一个 plist 块反序列化为 GPUSnapshot。
// 使用 plist.NewDecoder 与 encoding/json 风格一致的 API。
// 字段缺失时返回零值；输入非法 plist 时返回 error。
func ParseSample(data []byte) (*GPUSnapshot, error) {
	var sample PowerMetricsSample
	if err := plist.NewDecoder(bytes.NewReader(data)).Decode(&sample); err != nil {
		return nil, err
	}

	return &GPUSnapshot{
		ActiveResidency: sample.GPU.ActiveResidency(),
		FreqMHz:         sample.GPU.FreqMHz,
		PowerMW:         sample.Processor.GPUPowerMW,
		ThermalPressure: sample.ThermalPressure,
		SampledAt:       sample.Timestamp,
	}, nil
}
