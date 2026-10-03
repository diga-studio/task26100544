package dcim

import "sort"

// Plan 返回一个接纳设备数量全局最大的上架方案。
// 未出现在返回值中的请求视为未接纳；同样最优的方案允许任选。
//
// 方法：单棵分支限界搜索树。设备按高度降序，每台设备尝试
// “放入某段空闲区间”或“拒绝”，用 accepted+remaining <= best 剪枝；
// 同柜等长区间去重、相同规格设备的拒绝构成后缀，消除对称分支。
// 搜索完整棵树后 best 即为全局最大接纳数，规模上限
// （16 请求 / 8 机柜 / 24 区间 / 4 UPS）下远快于 10 秒。
func Plan(snapshot Snapshot, requests []Request) []Placement {
	if len(requests) == 0 {
		return nil
	}
	s := newSolver(snapshot, requests)
	s.greedyInit()
	s.dfs(0, 0)
	return s.bestPlan
}

// intervalRef 把空闲区间扁平化并关联到机柜。
type intervalRef struct {
	rack  int
	start int
	end   int
}

const (
	assignUnplaced = -1
	assignRejected = -2
)

type solver struct {
	racks     []Rack
	intervals []intervalRef
	upsRemain map[string]int
	devs      []Request // 按高度降序，提高剪枝效率

	ivUsed []int          // 每个区间已用 U 数
	rackW  []int          // 每柜已用重量
	rackP  []int          // 每柜已用功率
	upsUse map[string]int // 每 UPS 已用功率
	assign []int          // devs[i] 的区间下标 / assignRejected

	// 每个后缀 devs[pos:] 的资源需求升序序列，用于聚合上界剪枝。
	suffixH [][]int
	suffixW [][]int
	suffixP [][]int

	best     int
	bestPlan []Placement
}

func newSolver(snapshot Snapshot, requests []Request) *solver {
	s := &solver{
		racks:     snapshot.Racks,
		upsRemain: make(map[string]int, len(snapshot.UPS)),
		upsUse:    make(map[string]int, len(snapshot.UPS)),
	}
	for _, u := range snapshot.UPS {
		s.upsRemain[u.ID] = u.PowerRemainW
	}
	for ri, rk := range snapshot.Racks {
		for _, iv := range rk.Free {
			s.intervals = append(s.intervals, intervalRef{rack: ri, start: iv.Start, end: iv.End})
		}
	}
	s.ivUsed = make([]int, len(s.intervals))
	s.rackW = make([]int, len(s.racks))
	s.rackP = make([]int, len(s.racks))

	s.devs = make([]Request, len(requests))
	copy(s.devs, requests)
	sort.SliceStable(s.devs, func(i, j int) bool { return s.devs[i].HeightU > s.devs[j].HeightU })
	s.assign = make([]int, len(s.devs))
	for i := range s.assign {
		s.assign[i] = assignUnplaced
	}
	s.suffixH = make([][]int, len(s.devs)+1)
	s.suffixW = make([][]int, len(s.devs)+1)
	s.suffixP = make([][]int, len(s.devs)+1)
	for pos := len(s.devs) - 1; pos >= 0; pos-- {
		d := s.devs[pos]
		s.suffixH[pos] = insertSorted(s.suffixH[pos+1], d.HeightU)
		s.suffixW[pos] = insertSorted(s.suffixW[pos+1], d.Weight)
		s.suffixP[pos] = insertSorted(s.suffixP[pos+1], d.Power)
	}
	return s
}

func insertSorted(sorted []int, v int) []int {
	out := make([]int, len(sorted)+1)
	i := sort.SearchInts(sorted, v)
	copy(out, sorted[:i])
	out[i] = v
	copy(out[i+1:], sorted[i:])
	return out
}

// greedyInit 用几种排序启发式各跑一遍首次适应，
// 把最好的结果作为分支限界的初始 incumbent，加速最优性证明。
func (s *solver) greedyInit() {
	n := len(s.devs)
	orders := [][]int{
		sortedIndices(s.devs, func(a, b Request) bool { return a.Power < b.Power }),
		sortedIndices(s.devs, func(a, b Request) bool { return a.HeightU > b.HeightU }),
		sortedIndices(s.devs, func(a, b Request) bool { return a.Weight > b.Weight }),
	}
	for _, order := range orders {
		// 克隆资源状态
		ivUsed := make([]int, len(s.intervals))
		rackW := make([]int, len(s.racks))
		rackP := make([]int, len(s.racks))
		upsUse := make(map[string]int, len(s.upsRemain))
		assign := make([]int, n)
		for i := range assign {
			assign[i] = assignUnplaced
		}
		accepted := 0
		for _, di := range order {
			d := s.devs[di]
			for ii, iv := range s.intervals {
				if iv.end-iv.start+1-ivUsed[ii] < d.HeightU {
					continue
				}
				ri := iv.rack
				rk := s.racks[ri]
				if rackW[ri]+d.Weight > rk.WeightRemainLb ||
					rackP[ri]+d.Power > rk.PowerRemainW ||
					upsUse[rk.UPS]+d.Power > s.upsRemain[rk.UPS] {
					continue
				}
				ivUsed[ii] += d.HeightU
				rackW[ri] += d.Weight
				rackP[ri] += d.Power
				upsUse[rk.UPS] += d.Power
				assign[di] = ii
				accepted++
				break
			}
		}
		if accepted > s.best {
			s.best = accepted
			s.bestPlan = s.placementsFromAssign(assign)
		}
	}
}

func sortedIndices(devs []Request, less func(a, b Request) bool) []int {
	idx := make([]int, len(devs))
	for i := range idx {
		idx[i] = i
	}
	sort.SliceStable(idx, func(i, j int) bool { return less(devs[idx[i]], devs[idx[j]]) })
	return idx
}

// dfs 决定 devs[pos] 的放置或拒绝，accepted 为已接纳台数。
func (s *solver) dfs(pos, accepted int) {
	if accepted+len(s.devs)-pos <= s.best {
		return
	}
	if accepted+s.aggregateBound(pos) <= s.best {
		return
	}
	if pos == len(s.devs) {
		s.best = accepted
		s.bestPlan = s.buildPlacements()
		return
	}
	d := s.devs[pos]

	// 相同规格连续设备的放置按区间下标单调不减，消除同规格排列对称。
	minInterval := 0
	if pos > 0 && s.devs[pos-1] == d && s.assign[pos-1] >= 0 {
		minInterval = s.assign[pos-1]
	}

	// 分支一：放入某段空闲区间。同柜、当前剩余长度相同的区间等价，只试第一个。
	type seenKey struct{ rack, remain int }
	seen := make(map[seenKey]bool)
	for ii := minInterval; ii < len(s.intervals); ii++ {
		iv := s.intervals[ii]
		remain := iv.end - iv.start + 1 - s.ivUsed[ii]
		if remain < d.HeightU {
			continue
		}
		ri := iv.rack
		rk := s.racks[ri]
		if s.rackW[ri]+d.Weight > rk.WeightRemainLb ||
			s.rackP[ri]+d.Power > rk.PowerRemainW ||
			s.upsUse[rk.UPS]+d.Power > s.upsRemain[rk.UPS] {
			continue
		}
		key := seenKey{ri, remain}
		if seen[key] {
			continue
		}
		seen[key] = true

		s.ivUsed[ii] += d.HeightU
		s.rackW[ri] += d.Weight
		s.rackP[ri] += d.Power
		s.upsUse[rk.UPS] += d.Power
		s.assign[pos] = ii
		s.dfs(pos+1, accepted+1)
		s.ivUsed[ii] -= d.HeightU
		s.rackW[ri] -= d.Weight
		s.rackP[ri] -= d.Power
		s.upsUse[rk.UPS] -= d.Power
		s.assign[pos] = assignUnplaced
	}

	// 分支二：拒绝。相同规格连续设备的拒绝必须构成后缀，
	// 即前一台同规格设备若已被拒绝，才允许拒绝本台，避免重复枚举。
	if pos == 0 || s.devs[pos-1] != d || s.assign[pos-1] == assignRejected {
		s.assign[pos] = assignRejected
		s.dfs(pos+1, accepted)
		s.assign[pos] = assignUnplaced
	}
}

// aggregateBound 估计 devs[pos:] 中最多还能接纳多少台：
// 分别按剩余总 U 数、机柜总承重、机柜总功率、UPS 总功率，
// 取需求最小的设备优先能装下的台数，取各资源的最小值。
func (s *solver) aggregateBound(pos int) int {
	capH := 0
	for i, iv := range s.intervals {
		capH += iv.end - iv.start + 1 - s.ivUsed[i]
	}
	capW := 0
	capP := 0
	for i, rk := range s.racks {
		capW += rk.WeightRemainLb - s.rackW[i]
		capP += rk.PowerRemainW - s.rackP[i]
	}
	capUPS := 0
	for id, remain := range s.upsRemain {
		capUPS += remain - s.upsUse[id]
	}
	bound := countFitting(s.suffixH[pos], capH)
	if b := countFitting(s.suffixW[pos], capW); b < bound {
		bound = b
	}
	if b := countFitting(s.suffixP[pos], capP); b < bound {
		bound = b
	}
	if b := countFitting(s.suffixP[pos], capUPS); b < bound {
		bound = b
	}
	return bound
}

// countFitting 返回升序需求序列中总和不超 capacity 的最长前缀长度。
func countFitting(needs []int, capacity int) int {
	sum := 0
	for i, v := range needs {
		sum += v
		if sum > capacity {
			return i
		}
	}
	return len(needs)
}

// buildPlacements 由当前 assign 构造占位方案：
// 同一区间内的设备从区间底部向上依次堆叠。
func (s *solver) buildPlacements() []Placement {
	return s.placementsFromAssign(s.assign)
}

func (s *solver) placementsFromAssign(assign []int) []Placement {
	nextFree := make([]int, len(s.intervals))
	for i, iv := range s.intervals {
		nextFree[i] = iv.start
	}
	var placements []Placement
	for i, d := range s.devs {
		ivIdx := assign[i]
		if ivIdx < 0 {
			continue
		}
		iv := s.intervals[ivIdx]
		placements = append(placements, Placement{
			RequestID: d.ID,
			RackID:    s.racks[iv.rack].ID,
			StartU:    nextFree[ivIdx],
		})
		nextFree[ivIdx] += d.HeightU
	}
	return placements
}
