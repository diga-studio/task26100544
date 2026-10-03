package dcim

import (
	"math/rand"
	"testing"
)

// bruteForceMax 独立的穷举参照：枚举所有请求子集，
// 对每个子集枚举每台设备的具体 (机柜, 起始U) 占位并直接检查全部约束，
// 返回最大可接纳数量。与库内求解器不共享任何逻辑。
func bruteForceMax(snap Snapshot, reqs []Request) int {
	best := 0
	n := len(reqs)
	for mask := 1; mask < (1 << n); mask++ {
		var sub []Request
		for i := 0; i < n; i++ {
			if mask&(1<<i) != 0 {
				sub = append(sub, reqs[i])
			}
		}
		if len(sub) <= best {
			continue
		}
		if bruteFeasible(snap, sub) {
			best = len(sub)
		}
	}
	return best
}

// bruteFeasible 枚举子集中每台设备的所有合法起始 U 位组合。
type bruteSlot struct{ rack, start int }

func bruteFeasible(snap Snapshot, devs []Request) bool {
	options := make([][]bruteSlot, len(devs))
	for i, d := range devs {
		for ri, rk := range snap.Racks {
			for _, iv := range rk.Free {
				for st := iv.Start; st+d.HeightU-1 <= iv.End; st++ {
					options[i] = append(options[i], bruteSlot{ri, st})
				}
			}
		}
		if len(options[i]) == 0 {
			return false
		}
	}
	chosen := make([]bruteSlot, len(devs))
	var rec func(pos int) bool
	rec = func(pos int) bool {
		if pos == len(devs) {
			return true
		}
		for _, sl := range options[pos] {
			chosen[pos] = sl
			if partialOK(snap, devs, chosen[:pos+1]) && rec(pos+1) {
				return true
			}
		}
		return false
	}
	return rec(0)
}

// partialOK 检查前 k 台已定位设备之间及与快照约束的一致性。
func partialOK(snap Snapshot, devs []Request, chosen []bruteSlot) bool {
	weight := map[int]int{}
	power := map[int]int{}
	upsPower := map[string]int{}
	for i, d := range devs[:len(chosen)] {
		sl := chosen[i]
		rk := snap.Racks[sl.rack]
		// 与其他设备的 U 位重叠
		for j := 0; j < i; j++ {
			ol := chosen[j]
			if ol.rack != sl.rack {
				continue
			}
			aLo, aHi := sl.start, sl.start+d.HeightU-1
			bLo, bHi := ol.start, ol.start+devs[j].HeightU-1
			if aLo <= bHi && bLo <= aHi {
				return false
			}
		}
		weight[sl.rack] += d.Weight
		power[sl.rack] += d.Power
		upsPower[rk.UPS] += d.Power
		if weight[sl.rack] > rk.WeightRemainLb || power[sl.rack] > rk.PowerRemainW {
			return false
		}
	}
	remain := map[string]int{}
	for _, u := range snap.UPS {
		remain[u.ID] = u.PowerRemainW
	}
	for id, p := range upsPower {
		if p > remain[id] {
			return false
		}
	}
	return true
}

// TestBruteForceCrossCheck 小规模随机实例上，
// 用独立穷举核对 Plan 的接纳数量，并用 Validate 检查每个返回方案。
func TestBruteForceCrossCheck(t *testing.T) {
	rng := rand.New(rand.NewSource(20261005))
	for trial := 0; trial < 300; trial++ {
		nRack := 1 + rng.Intn(2)
		nUPS := 1 + rng.Intn(2)
		ups := make([]UPS, nUPS)
		for i := range ups {
			ups[i] = UPS{ID: string(rune('A' + i)), PowerRemainW: 200 + rng.Intn(800)}
		}
		racks := make([]Rack, nRack)
		for i := range racks {
			racks[i] = Rack{
				ID:             string(rune('a' + i)),
				HeightU:        6,
				UPS:            ups[rng.Intn(nUPS)].ID,
				WeightRemainLb: 100 + rng.Intn(400),
				PowerRemainW:   200 + rng.Intn(600),
			}
			// 随机切 1~2 段不相交空闲区间
			lo := 1 + rng.Intn(3)
			hi := lo + rng.Intn(6-lo)
			racks[i].Free = append(racks[i].Free, Interval{Start: lo, End: hi})
			if hi+2 <= 6 && rng.Intn(2) == 0 {
				lo2 := hi + 1 + rng.Intn(6-hi)
				hi2 := lo2 + rng.Intn(7-lo2)
				racks[i].Free = append(racks[i].Free, Interval{Start: lo2, End: hi2})
			}
		}
		snap := Snapshot{Racks: racks, UPS: ups}

		nReq := 2 + rng.Intn(5) // 2~6 台
		reqs := make([]Request, nReq)
		for i := range reqs {
			reqs[i] = Request{
				ID:      string(rune('0' + i)),
				HeightU: 1 + rng.Intn(3),
				Weight:  50 + rng.Intn(200),
				Power:   50 + rng.Intn(300),
			}
		}

		got := Plan(snap, reqs)
		want := bruteForceMax(snap, reqs)
		if len(got) != want {
			t.Fatalf("trial %d: Plan 接纳 %d 台，穷举最大 %d 台\nsnap=%+v\nreqs=%+v\nplan=%v",
				trial, len(got), want, snap, reqs, got)
		}
		if err := Validate(snap, reqs, got); err != nil {
			t.Fatalf("trial %d: Validate 拒绝了 Plan 的方案: %v\nplan=%v", trial, err, got)
		}
	}
}
