package macgpu

import (
	"testing"
	"time"
)

// m1PlistFixture 是一段简化的 M1 powermetrics plist 输出样本，
// 基于真实采样结构手工精简（保留 GPU 相关字段）。
const m1PlistFixture = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>elapsed_ns</key>
	<integer>2019178750</integer>
	<key>hw_model</key>
	<string>MacBookPro17,1</string>
	<key>timestamp</key>
	<string>2024-01-01T12:00:00Z</string>
	<key>thermal_pressure</key>
	<string>Nominal</string>
	<key>processor</key>
	<dict>
		<key>gpu_power</key>
		<real>15.3528</real>
		<key>gpu_energy</key>
		<integer>31</integer>
		<key>cpu_power</key>
		<real>44.0773</real>
		<key>ane_power</key>
		<real>0.0</real>
		<key>combined_power</key>
		<real>59.4301</real>
	</dict>
	<key>gpu</key>
	<dict>
		<key>freq_hz</key>
		<real>714.836</real>
		<key>idle_ratio</key>
		<real>0.983341</real>
		<key>idle_ns</key>
		<integer>1980028458</integer>
		<key>gpu_energy</key>
		<integer>31</integer>
		<key>dvfm_states</key>
		<array>
			<dict>
				<key>freq</key>
				<integer>396</integer>
				<key>used_ratio</key>
				<real>0.000265531</real>
				<key>used_ns</key>
				<integer>534666</integer>
			</dict>
			<dict>
				<key>freq</key>
				<integer>720</integer>
				<key>used_ratio</key>
				<real>0.0163933</real>
				<key>used_ns</key>
				<integer>33009083</integer>
			</dict>
		</array>
	</dict>
</dict>
</plist>`

// idleGPUPlistFixture 模拟 GPU 完全空闲的情况（idle_ratio = 1.0）。
const idleGPUPlistFixture = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>elapsed_ns</key>
	<integer>1000000000</integer>
	<key>thermal_pressure</key>
	<string>Nominal</string>
	<key>processor</key>
	<dict>
		<key>gpu_power</key>
		<real>0.0</real>
	</dict>
	<key>gpu</key>
	<dict>
		<key>freq_hz</key>
		<real>0.0</real>
		<key>idle_ratio</key>
		<real>1.0</real>
		<key>dvfm_states</key>
		<array/>
	</dict>
</dict>
</plist>`

// fullGPUPlistFixture 模拟 GPU 满载情况（idle_ratio = 0.0）。
const fullGPUPlistFixture = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>elapsed_ns</key>
	<integer>1000000000</integer>
	<key>thermal_pressure</key>
	<string>Heavy</string>
	<key>processor</key>
	<dict>
		<key>gpu_power</key>
		<real>8000.0</real>
	</dict>
	<key>gpu</key>
	<dict>
		<key>freq_hz</key>
		<real>1296.0</real>
		<key>idle_ratio</key>
		<real>0.0</real>
		<key>dvfm_states</key>
		<array/>
	</dict>
</dict>
</plist>`

// noThermalPlistFixture 模拟 thermal_pressure 字段缺失的情况。
const noThermalPlistFixture = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>elapsed_ns</key>
	<integer>1000000000</integer>
	<key>processor</key>
	<dict>
		<key>gpu_power</key>
		<real>5.0</real>
	</dict>
	<key>gpu</key>
	<dict>
		<key>freq_hz</key>
		<real>396.0</real>
		<key>idle_ratio</key>
		<real>0.5</real>
		<key>dvfm_states</key>
		<array/>
	</dict>
</dict>
</plist>`

func TestParseSample(t *testing.T) {
	tests := []struct {
		name            string
		input           []byte
		wantErr         bool
		wantResidency   float64
		wantFreqMHz     float64
		wantPowerMW     float64
		wantThermal     string
		wantSampledAtOK bool
	}{
		{
			name:            "M1 正常采样",
			input:           []byte(m1PlistFixture),
			wantErr:         false,
			wantResidency:   1.0 - 0.983341,
			wantFreqMHz:     714.836,
			wantPowerMW:     15.3528,
			wantThermal:     "Nominal",
			wantSampledAtOK: true,
		},
		{
			name:          "GPU 完全空闲（idle_ratio=1.0）不返回负值",
			input:         []byte(idleGPUPlistFixture),
			wantErr:       false,
			wantResidency: 0.0,
			wantFreqMHz:   0.0,
			wantPowerMW:   0.0,
			wantThermal:   "Nominal",
		},
		{
			name:          "GPU 满载（idle_ratio=0.0）返回 1.0",
			input:         []byte(fullGPUPlistFixture),
			wantErr:       false,
			wantResidency: 1.0,
			wantFreqMHz:   1296.0,
			wantPowerMW:   8000.0,
			wantThermal:   "Heavy",
		},
		{
			name:          "thermal_pressure 字段缺失不 panic",
			input:         []byte(noThermalPlistFixture),
			wantErr:       false,
			wantResidency: 0.5,
			wantFreqMHz:   396.0,
			wantPowerMW:   5.0,
			wantThermal:   "",
		},
		{
			name:    "非法输入返回 error",
			input:   []byte("this is not a plist"),
			wantErr: true,
		},
		{
			// howett.net/plist 对空输入返回零值而非 error；空输入在实际场景中
			// 不会出现（NUL 分隔块至少包含 XML 头），这里验证不 panic 即可。
			// idle_ratio 零值为 0.0，故 ActiveResidency = 1 - 0 = 1.0。
			name:          "空输入不 panic，返回零值（idle_ratio=0 → residency=1）",
			input:         []byte{},
			wantErr:       false,
			wantResidency: 1.0,
			wantFreqMHz:   0.0,
			wantPowerMW:   0.0,
			wantThermal:   "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseSample(tc.input)

			if tc.wantErr {
				if err == nil {
					t.Errorf("期望返回 error，但 err == nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("意外错误: %v", err)
			}

			const eps = 1e-6
			if diff := got.ActiveResidency - tc.wantResidency; diff < -eps || diff > eps {
				t.Errorf("ActiveResidency = %v，期望 %v", got.ActiveResidency, tc.wantResidency)
			}
			if diff := got.FreqMHz - tc.wantFreqMHz; diff < -eps || diff > eps {
				t.Errorf("FreqMHz = %v，期望 %v", got.FreqMHz, tc.wantFreqMHz)
			}
			if diff := got.PowerMW - tc.wantPowerMW; diff < -eps || diff > eps {
				t.Errorf("PowerMW = %v，期望 %v", got.PowerMW, tc.wantPowerMW)
			}
			if got.ThermalPressure != tc.wantThermal {
				t.Errorf("ThermalPressure = %q，期望 %q", got.ThermalPressure, tc.wantThermal)
			}
			if tc.wantSampledAtOK && got.SampledAt.IsZero() {
				t.Errorf("期望 SampledAt 非零值，但得到零值")
			}
		})
	}
}

func TestGPUStatsActiveResidency(t *testing.T) {
	tests := []struct {
		name      string
		idleRatio float64
		want      float64
	}{
		{"正常占用", 0.3, 0.7},
		{"完全空闲", 1.0, 0.0},
		{"满载", 0.0, 1.0},
		{"超出上界（防御）", -0.1, 1.0},
		{"超出下界（防御）", 1.1, 0.0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := &GPUStats{IdleRatio: tc.idleRatio}
			got := g.ActiveResidency()
			const eps = 1e-9
			if diff := got - tc.want; diff < -eps || diff > eps {
				t.Errorf("ActiveResidency() = %v，期望 %v", got, tc.want)
			}
		})
	}
}

func TestParseSampleTimestamp(t *testing.T) {
	got, err := ParseSample([]byte(m1PlistFixture))
	if err != nil {
		t.Fatal(err)
	}
	expected := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	if !got.SampledAt.Equal(expected) {
		t.Errorf("SampledAt = %v，期望 %v", got.SampledAt, expected)
	}
}
