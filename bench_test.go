package dcim

import (
	"testing"
	"time"
)

// fullScaleSnapshot 满规模固定输入：8 机柜、24 段空闲区间、4 个 UPS。
// 机柜功率余量宽松，约束主要来自碎片化连续空间、机柜承重与 UPS 共享功率。
func fullScaleSnapshot() Snapshot {
	ups := []UPS{
		{ID: "UPS-A", PowerRemainW: 1500},
		{ID: "UPS-B", PowerRemainW: 1200},
		{ID: "UPS-C", PowerRemainW: 800},
		{ID: "UPS-D", PowerRemainW: 600},
	}
	// 每个机柜 3 段空闲区间，共 24 段；间隙形状各异，最大连续段 3~8U。
	freeSets := [][]Interval{
		{{1, 6}, {8, 12}, {14, 16}}, // 6+5+3
		{{1, 3}, {5, 9}, {11, 13}},  // 3+5+3
		{{1, 8}, {10, 12}, {14, 15}},
		{{1, 4}, {6, 10}, {12, 14}},
		{{1, 5}, {7, 11}, {13, 15}},
		{{1, 2}, {4, 9}, {11, 14}},
		{{1, 7}, {9, 11}, {13, 14}},
		{{1, 3}, {5, 8}, {10, 14}},
	}
	racks := make([]Rack, 8)
	for i := range racks {
		racks[i] = Rack{
			ID:             "R" + string(rune('1'+i)),
			HeightU:        42,
			UPS:            ups[i%4].ID,
			Free:           freeSets[i],
			WeightRemainLb: 1200 + 100*i, // 1200~1900，承重也会约束
			PowerRemainW:   5000,
		}
	}
	return Snapshot{Racks: racks, UPS: ups}
}

// fullScaleRequests 16 台设备：4 种规格各 4 台（含重复规格），
// 总功率 6800W 远超 UPS 合计 4100W，不可能全部接纳。
func fullScaleRequests() []Request {
	specs := []struct {
		name           string
		h, w, p, count int
	}{
		{"srv1u", 1, 100, 200, 4},
		{"srv2u", 2, 150, 300, 4},
		{"stor4u", 4, 400, 500, 4},
		{"gpu6u", 6, 600, 700, 4},
	}
	var reqs []Request
	for _, sp := range specs {
		for i := 0; i < sp.count; i++ {
			reqs = append(reqs, Request{
				ID:      sp.name + "-" + string(rune('a'+i)),
				HeightU: sp.h, Weight: sp.w, Power: sp.p,
			})
		}
	}
	return reqs
}

// TestFullScale 固定满规模输入：断言不能全部接纳、方案合法、
// 接纳数等于已核定的最优值，并记录本机耗时。
func TestFullScale(t *testing.T) {
	snap := fullScaleSnapshot()
	reqs := fullScaleRequests()

	start := time.Now()
	plan := Plan(snap, reqs)
	elapsed := time.Since(start)
	t.Logf("满规模求解耗时: %v，接纳 %d/16 台", elapsed, len(plan))

	if len(plan) == len(reqs) {
		t.Fatal("该基准不应全部接纳")
	}
	const wantOptimal = 12 // 最优接纳数：UPS 总余量 4100W 最多容纳 12 台最小功率设备
	if len(plan) != wantOptimal {
		t.Fatalf("接纳数 = %d，期望最优值 %d", len(plan), wantOptimal)
	}
	if err := Validate(snap, reqs, plan); err != nil {
		t.Fatalf("Validate 拒绝了 Plan 的方案: %v", err)
	}
	if elapsed > 10*time.Second {
		t.Fatalf("求解耗时 %v 超过 10 秒", elapsed)
	}
}

// BenchmarkPlanFullScale 满规模固定输入基准。
func BenchmarkPlanFullScale(b *testing.B) {
	snap := fullScaleSnapshot()
	reqs := fullScaleRequests()
	for i := 0; i < b.N; i++ {
		Plan(snap, reqs)
	}
}
