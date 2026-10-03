package rackplan

import "sort"

// Plan 返回一个接纳设备数量达到全局最大值的上架方案。
// 未被接纳的请求不会出现在返回值中；同样最优的方案允许任选。
//
// 算法：精确分支限界。请求按高度降序处理，每台设备尝试放入各机柜
// 当前空闲区间的左端（一维装箱必存在左对齐可行解，故候选位置完备），
// 或尝试跳过。剪枝上界取三者最小：剩余请求数、逐台可放台数、
// 按聚合资源（UPS 总功率、机柜总功率、总承重、总空闲 U）可再容纳的台数。
// 不拆分设备、不挪动现有资产，机柜承重/功率与 UPS 共享功率在搜索中增量维护。
func Plan(snapshot Snapshot, requests []Request) []Placement {
	s := newSolver(snapshot, requests)
	s.search(0, 0)
	return s.best
}

type rackState struct {
	id     string
	free   []Interval // 残余空闲区间；放置即左端收缩，空前段自然失效
	weight int        // 剩余承重
	power  int        // 机柜剩余功率
	ups    int        // 所属 UPS 下标
}

type solver struct {
	reqs   []Request // 已按高度降序排序
	racks  []rackState
	upsRem []int
	cur    []Placement
	best   []Placement
}

func newSolver(snapshot Snapshot, requests []Request) *solver {
	upsIndex := make(map[string]int, len(snapshot.UPSs))
	upsRem := make([]int, len(snapshot.UPSs))
	for i, u := range snapshot.UPSs {
		upsIndex[u.ID] = i
		upsRem[i] = u.RemainingPowerW
	}
	racks := make([]rackState, len(snapshot.Racks))
	for i, r := range snapshot.Racks {
		free := make([]Interval, len(r.Free))
		copy(free, r.Free)
		racks[i] = rackState{
			id:     r.ID,
			free:   free,
			weight: r.RemainingWeightLB,
			power:  r.RemainingPowerW,
			ups:    upsIndex[r.UPSID],
		}
	}
	reqs := make([]Request, len(requests))
	copy(reqs, requests)
	sort.SliceStable(reqs, func(a, b int) bool {
		x, y := reqs[a], reqs[b]
		if x.HeightU != y.HeightU {
			return x.HeightU > y.HeightU
		}
		if x.WeightLB != y.WeightLB {
			return x.WeightLB > y.WeightLB
		}
		if x.PowerW != y.PowerW {
			return x.PowerW > y.PowerW
		}
		return x.ID < y.ID
	})
	return &solver{reqs: reqs, racks: racks, upsRem: upsRem}
}

func (s *solver) search(i, accepted int) {
	remaining := len(s.reqs) - i
	if accepted+remaining <= len(s.best) {
		return
	}
	if bound := s.upperBound(i); accepted+bound <= len(s.best) {
		return
	}
	if i == len(s.reqs) {
		s.best = append([]Placement(nil), s.cur...)
		return
	}
	req := s.reqs[i]
	for ri := range s.racks {
		r := &s.racks[ri]
		if req.WeightLB > r.weight || req.PowerW > r.power || req.PowerW > s.upsRem[r.ups] {
			continue
		}
		for ii := range r.free {
			iv := r.free[ii]
			if iv.EndU-iv.StartU+1 < req.HeightU {
				continue
			}
			// 左对齐放置：区间左端收缩，恢复时还原。
			r.free[ii].StartU += req.HeightU
			r.weight -= req.WeightLB
			r.power -= req.PowerW
			s.upsRem[r.ups] -= req.PowerW
			s.cur = append(s.cur, Placement{RequestID: req.ID, RackID: r.id, StartU: iv.StartU})
			s.search(i+1, accepted+1)
			s.cur = s.cur[:len(s.cur)-1]
			s.upsRem[r.ups] += req.PowerW
			r.power += req.PowerW
			r.weight += req.WeightLB
			r.free[ii].StartU = iv.StartU
		}
	}
	s.search(i+1, accepted) // 跳过该请求的分支
}

// upperBound 返回从下标 i 起还能接纳台数的上界，取逐台可放台数与
// 聚合资源可容台数的最小值（均不考虑彼此竞争，故为上界）。
func (s *solver) upperBound(i int) int {
	bound := s.fitBound(i)
	if bound == 0 {
		return 0
	}
	rest := s.reqs[i:]
	// 聚合资源上界：任一可行 m 台子集的总消耗不少于 m 台最小消耗之和。
	totalUPSPower, totalRackPower, totalWeight, totalFreeU := 0, 0, 0, 0
	for _, u := range s.upsRem {
		totalUPSPower += u
	}
	for ri := range s.racks {
		r := &s.racks[ri]
		totalRackPower += r.power
		totalWeight += r.weight
		for _, iv := range r.free {
			totalFreeU += iv.EndU - iv.StartU + 1
		}
	}
	powers := make([]int, len(rest))
	weights := make([]int, len(rest))
	heights := make([]int, len(rest))
	for j, req := range rest {
		powers[j] = req.PowerW
		weights[j] = req.WeightLB
		heights[j] = req.HeightU
	}
	for _, b := range [][2]int{
		{greedyCount(powers, totalUPSPower), greedyCount(powers, totalRackPower)},
		{greedyCount(weights, totalWeight), greedyCount(heights, totalFreeU)},
	} {
		if b[0] < bound {
			bound = b[0]
		}
		if b[1] < bound {
			bound = b[1]
		}
	}
	return bound
}

// greedyCount 返回总容量 capacity 内最多能装下的台数（按消耗升序取）。
func greedyCount(vals []int, capacity int) int {
	sort.Ints(vals)
	count, sum := 0, 0
	for _, v := range vals {
		if sum+v > capacity {
			break
		}
		sum += v
		count++
	}
	return count
}

// fitBound 返回从下标 i 起的请求中，当前状态下逐台各自可放的台数。
func (s *solver) fitBound(i int) int {
	bound := 0
	for j := i; j < len(s.reqs); j++ {
		if s.fitsSomewhere(s.reqs[j]) {
			bound++
		}
	}
	return bound
}

func (s *solver) fitsSomewhere(req Request) bool {
	for ri := range s.racks {
		r := &s.racks[ri]
		if req.WeightLB > r.weight || req.PowerW > r.power || req.PowerW > s.upsRem[r.ups] {
			continue
		}
		for _, iv := range r.free {
			if iv.EndU-iv.StartU+1 >= req.HeightU {
				return true
			}
		}
	}
	return false
}
