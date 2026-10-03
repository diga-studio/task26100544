package rackplan

import (
	"testing"
	"time"
)

// fullScaleFixture 返回满规模固定输入：16 个请求、8 个机柜、
// 24 段空闲区间、4 个 UPS，含多种高度/重量/功率与重复规格。
// 全部请求合计 7500W，超出 4 个 UPS 合计 6700W 的余量，
// 因此不可能全部接纳；最大可接纳数为 15。
func fullScaleFixture() (Snapshot, []Request) {
	snap := Snapshot{
		Racks: []Rack{
			{ID: "R1", HeightU: 48, Free: []Interval{{2, 9}, {14, 20}, {25, 33}}, RemainingWeightLB: 1200, RemainingPowerW: 2500, UPSID: "U1"},
			{ID: "R2", HeightU: 42, Free: []Interval{{1, 6}, {10, 15}, {20, 24}}, RemainingWeightLB: 900, RemainingPowerW: 2000, UPSID: "U1"},
			{ID: "R3", HeightU: 48, Free: []Interval{{3, 10}, {15, 22}, {27, 35}}, RemainingWeightLB: 1500, RemainingPowerW: 3000, UPSID: "U2"},
			{ID: "R4", HeightU: 42, Free: []Interval{{1, 5}, {9, 14}, {18, 22}}, RemainingWeightLB: 800, RemainingPowerW: 1800, UPSID: "U2"},
			{ID: "R5", HeightU: 52, Free: []Interval{{2, 11}, {16, 24}, {30, 38}}, RemainingWeightLB: 2000, RemainingPowerW: 3500, UPSID: "U3"},
			{ID: "R6", HeightU: 48, Free: []Interval{{1, 7}, {12, 17}, {22, 28}}, RemainingWeightLB: 1100, RemainingPowerW: 2400, UPSID: "U3"},
			{ID: "R7", HeightU: 42, Free: []Interval{{4, 9}, {14, 19}, {24, 30}}, RemainingWeightLB: 1000, RemainingPowerW: 2000, UPSID: "U4"},
			{ID: "R8", HeightU: 42, Free: []Interval{{1, 4}, {8, 13}, {17, 21}}, RemainingWeightLB: 700, RemainingPowerW: 1600, UPSID: "U4"},
		},
		UPSs: []UPS{
			{ID: "U1", RemainingPowerW: 1800},
			{ID: "U2", RemainingPowerW: 1500},
			{ID: "U3", RemainingPowerW: 2200},
			{ID: "U4", RemainingPowerW: 1200},
		},
	}
	reqs := []Request{
		{ID: "gpu-1", HeightU: 4, WeightLB: 250, PowerW: 900},
		{ID: "gpu-2", HeightU: 4, WeightLB: 250, PowerW: 900}, // 与 gpu-1 同规格
		{ID: "srv-a1", HeightU: 2, WeightLB: 80, PowerW: 450},
		{ID: "srv-a2", HeightU: 2, WeightLB: 80, PowerW: 450}, // 重复规格
		{ID: "srv-a3", HeightU: 2, WeightLB: 80, PowerW: 450}, // 重复规格
		{ID: "srv-b1", HeightU: 1, WeightLB: 45, PowerW: 300},
		{ID: "srv-b2", HeightU: 1, WeightLB: 45, PowerW: 300}, // 重复规格
		{ID: "srv-b3", HeightU: 1, WeightLB: 45, PowerW: 300}, // 重复规格
		{ID: "srv-b4", HeightU: 1, WeightLB: 45, PowerW: 300}, // 重复规格
		{ID: "sto-1", HeightU: 3, WeightLB: 150, PowerW: 650},
		{ID: "sto-2", HeightU: 3, WeightLB: 150, PowerW: 650}, // 重复规格
		{ID: "net-1", HeightU: 1, WeightLB: 30, PowerW: 150},
		{ID: "net-2", HeightU: 1, WeightLB: 30, PowerW: 150}, // 重复规格
		{ID: "blade-1", HeightU: 6, WeightLB: 320, PowerW: 1100},
		{ID: "mem-1", HeightU: 2, WeightLB: 60, PowerW: 250},
		{ID: "edge-1", HeightU: 1, WeightLB: 25, PowerW: 200},
	}
	return snap, reqs
}

// TestFullScale 在满规模固定输入上验证最优接纳数并记录本机耗时。
func TestFullScale(t *testing.T) {
	snap, reqs := fullScaleFixture()
	start := time.Now()
	got := Plan(snap, reqs)
	elapsed := time.Since(start)
	t.Logf("满规模求解耗时: %v（16 请求 / 8 机柜 / 24 区间 / 4 UPS）", elapsed)
	if err := Validate(snap, reqs, got); err != nil {
		t.Fatalf("满规模方案未通过 Validate: %v", err)
	}
	if len(got) != 15 {
		t.Fatalf("满规模固定输入的最大接纳数应为 15，实际 %d: %+v", len(got), got)
	}
	if elapsed > 10*time.Second {
		t.Fatalf("求解耗时 %v 超过 10 秒上限", elapsed)
	}
}

// BenchmarkFullScale 满规模固定输入基准（不含编译）。
func BenchmarkFullScale(b *testing.B) {
	snap, reqs := fullScaleFixture()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		got := Plan(snap, reqs)
		if len(got) != 15 {
			b.Fatalf("最大接纳数应为 15，实际 %d", len(got))
		}
	}
}
