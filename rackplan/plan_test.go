package rackplan

import (
	"math/rand"
	"testing"
)

// planAndCheck 运行 Plan 并先用 Validate 校验方案本身合法。
func planAndCheck(t *testing.T, snap Snapshot, reqs []Request) []Placement {
	t.Helper()
	got := Plan(snap, reqs)
	if err := Validate(snap, reqs, got); err != nil {
		t.Fatalf("Plan 返回的方案未通过 Validate: %v\n方案: %+v", err, got)
	}
	return got
}

// 总 U 数、重量、功率均有余量，但连续空间不足：机柜只有两段 1U 空闲，
// 2U 设备无处可放，必须接纳 0 台。
func TestPlanNoContiguousSpace(t *testing.T) {
	snap := Snapshot{
		Racks: []Rack{{
			ID: "R1", HeightU: 12,
			Free:              []Interval{{1, 1}, {3, 3}},
			RemainingWeightLB: 2000, RemainingPowerW: 5000, UPSID: "U1",
		}},
		UPSs: []UPS{{ID: "U1", RemainingPowerW: 10000}},
	}
	reqs := []Request{{ID: "srv-2u", HeightU: 2, WeightLB: 50, PowerW: 300}}
	got := planAndCheck(t, snap, reqs)
	if len(got) != 0 {
		t.Fatalf("连续空间不足时应接纳 0 台，实际 %d 台: %+v", len(got), got)
	}
}

// 每个机柜单独接纳都合法，但合并后超过共享 UPS 预算：最多接纳 1 台。
func TestPlanSharedUPSBudget(t *testing.T) {
	snap := Snapshot{
		Racks: []Rack{
			{ID: "R1", HeightU: 24, Free: []Interval{{1, 10}}, RemainingWeightLB: 1000, RemainingPowerW: 900, UPSID: "U1"},
			{ID: "R2", HeightU: 24, Free: []Interval{{1, 10}}, RemainingWeightLB: 1000, RemainingPowerW: 900, UPSID: "U1"},
		},
		UPSs: []UPS{{ID: "U1", RemainingPowerW: 1000}},
	}
	reqs := []Request{
		{ID: "a", HeightU: 2, WeightLB: 100, PowerW: 800},
		{ID: "b", HeightU: 2, WeightLB: 100, PowerW: 800},
	}
	got := planAndCheck(t, snap, reqs)
	if len(got) != 1 {
		t.Fatalf("共享 UPS 预算下应只接纳 1 台，实际 %d 台: %+v", len(got), got)
	}
}

// 先接纳大设备只能装 1 台，改选小设备可装 2 台：最优解必须选小设备。
func TestPlanPreferManySmall(t *testing.T) {
	snap := Snapshot{
		Racks: []Rack{{
			ID: "R1", HeightU: 12,
			Free:              []Interval{{1, 4}},
			RemainingWeightLB: 1000, RemainingPowerW: 5000, UPSID: "U1",
		}},
		UPSs: []UPS{{ID: "U1", RemainingPowerW: 10000}},
	}
	reqs := []Request{
		{ID: "big", HeightU: 4, WeightLB: 100, PowerW: 200},
		{ID: "small-1", HeightU: 2, WeightLB: 100, PowerW: 200},
		{ID: "small-2", HeightU: 2, WeightLB: 100, PowerW: 200},
	}
	got := planAndCheck(t, snap, reqs)
	if len(got) != 2 {
		t.Fatalf("应接纳 2 台小设备，实际 %d 台: %+v", len(got), got)
	}
	for _, p := range got {
		if p.RequestID == "big" {
			t.Fatalf("最优方案不应包含大设备: %+v", got)
		}
	}
}

// 重量预算约束：两台设备单独都合法，合计超重，只能接纳 1 台。
func TestPlanRackWeightBudget(t *testing.T) {
	snap := Snapshot{
		Racks: []Rack{{
			ID: "R1", HeightU: 24,
			Free:              []Interval{{1, 10}},
			RemainingWeightLB: 150, RemainingPowerW: 5000, UPSID: "U1",
		}},
		UPSs: []UPS{{ID: "U1", RemainingPowerW: 10000}},
	}
	reqs := []Request{
		{ID: "a", HeightU: 1, WeightLB: 100, PowerW: 100},
		{ID: "b", HeightU: 1, WeightLB: 100, PowerW: 100},
	}
	got := planAndCheck(t, snap, reqs)
	if len(got) != 1 {
		t.Fatalf("承重预算下应只接纳 1 台，实际 %d 台: %+v", len(got), got)
	}
}

// bruteForceMax 独立穷举：枚举所有请求子集，对每个子集用全位置
// （不限于左对齐）递归判定可行性，返回最大可接纳数量。
func bruteForceMax(snap Snapshot, reqs []Request) int {
	best := 0
	for mask := 0; mask < 1<<uint(len(reqs)); mask++ {
		var sub []Request
		for i := range reqs {
			if mask&(1<<uint(i)) != 0 {
				sub = append(sub, reqs[i])
			}
		}
		if len(sub) <= best || !feasible(snap, sub) {
			continue
		}
		best = len(sub)
	}
	return best
}

func feasible(snap Snapshot, sub []Request) bool {
	type rk struct {
		free          []Interval
		weight, power int
		ups           int
	}
	upsIndex := map[string]int{}
	var upsRem []int
	for _, u := range snap.UPSs {
		upsIndex[u.ID] = len(upsRem)
		upsRem = append(upsRem, u.RemainingPowerW)
	}
	racks := make([]rk, len(snap.Racks))
	for i, r := range snap.Racks {
		racks[i] = rk{append([]Interval(nil), r.Free...), r.RemainingWeightLB, r.RemainingPowerW, upsIndex[r.UPSID]}
	}
	var place func(k int) bool
	place = func(k int) bool {
		if k == len(sub) {
			return true
		}
		req := sub[k]
		for ri := range racks {
			r := &racks[ri]
			if req.WeightLB > r.weight || req.PowerW > r.power || req.PowerW > upsRem[r.ups] {
				continue
			}
			for _, iv := range r.free {
				// 尝试区间内每一个整数起始位，与 Plan 的左对齐优化无关。
				for s := iv.StartU; s+req.HeightU-1 <= iv.EndU; s++ {
					before := Interval{iv.StartU, s - 1}
					after := Interval{s + req.HeightU, iv.EndU}
					var next []Interval
					for _, o := range r.free {
						if o == iv {
							continue
						}
						next = append(next, o)
					}
					if before.StartU <= before.EndU {
						next = append(next, before)
					}
					if after.StartU <= after.EndU {
						next = append(next, after)
					}
					saved := r.free
					r.free = next
					r.weight -= req.WeightLB
					r.power -= req.PowerW
					upsRem[r.ups] -= req.PowerW
					ok := place(k + 1)
					upsRem[r.ups] += req.PowerW
					r.power += req.PowerW
					r.weight += req.WeightLB
					r.free = saved
					if ok {
						return true
					}
				}
			}
		}
		return false
	}
	return place(0)
}

// 小规模随机实例：用独立穷举核对 Plan 的接纳数量，并用 Validate 校验方案。
func TestPlanMatchesBruteForce(t *testing.T) {
	rng := rand.New(rand.NewSource(20261005))
	for trial := 0; trial < 300; trial++ {
		nUPS := 1 + rng.Intn(2)
		var upss []UPS
		for u := 0; u < nUPS; u++ {
			upss = append(upss, UPS{ID: string(rune('A' + u)), RemainingPowerW: 200 + rng.Intn(1500)})
		}
		nRacks := 1 + rng.Intn(3)
		var racks []Rack
		for r := 0; r < nRacks; r++ {
			height := 6 + rng.Intn(7)
			// 随机生成 1~3 段不重叠空闲区间。
			var free []Interval
			pos := 1
			for seg := 0; seg < 3 && pos <= height; seg++ {
				if rng.Intn(2) == 0 {
					pos++
					continue
				}
				length := 1 + rng.Intn(3)
				end := pos + length - 1
				if end > height {
					end = height
				}
				free = append(free, Interval{pos, end})
				pos = end + 1 + rng.Intn(2)
			}
			racks = append(racks, Rack{
				ID:                string(rune('a' + r)),
				HeightU:           height,
				Free:              free,
				RemainingWeightLB: 100 + rng.Intn(500),
				RemainingPowerW:   200 + rng.Intn(1500),
				UPSID:             upss[rng.Intn(nUPS)].ID,
			})
		}
		nReq := 1 + rng.Intn(6)
		var reqs []Request
		for q := 0; q < nReq; q++ {
			reqs = append(reqs, Request{
				ID:       string(rune('0' + q)),
				HeightU:  1 + rng.Intn(4),
				WeightLB: 20 + rng.Intn(200),
				PowerW:   50 + rng.Intn(600),
			})
		}
		snap := Snapshot{Racks: racks, UPSs: upss}
		got := planAndCheck(t, snap, reqs)
		want := bruteForceMax(snap, reqs)
		if len(got) != want {
			t.Fatalf("trial %d: Plan 接纳 %d 台，穷举最优 %d 台\n快照: %+v\n请求: %+v\n方案: %+v",
				trial, len(got), want, snap, reqs, got)
		}
	}
}
