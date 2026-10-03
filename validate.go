package dcim

import "fmt"

// Validate 独立检查方案是否满足全部安装约束，
// 不负责证明最优性，也不调用 Plan。
// 返回 nil 表示方案合法，否则返回描述首个违规处的错误。
//
// 检查项：
//   - 每个 placement 引用存在的请求与机柜；
//   - 每台设备最多被接纳一次；
//   - 设备完整落在该机柜某一段空闲闭区间内（不拆分、不跨段）；
//   - 同一机柜内设备不占相同 U 位；
//   - 每柜新设备重量之和不超过机柜剩余重量；
//   - 每柜新设备功率之和不超过机柜剩余功率；
//   - 每个 UPS 下所有机柜的新设备功率之和不超过 UPS 共享余量。
func Validate(snapshot Snapshot, requests []Request, placements []Placement) error {
	reqByID := make(map[string]Request, len(requests))
	for _, r := range requests {
		reqByID[r.ID] = r
	}
	rackIdx := make(map[string]int, len(snapshot.Racks))
	for i, rk := range snapshot.Racks {
		rackIdx[rk.ID] = i
	}
	upsRemain := make(map[string]int, len(snapshot.UPS))
	for _, u := range snapshot.UPS {
		upsRemain[u.ID] = u.PowerRemainW
	}

	accepted := make(map[string]bool, len(placements))
	weightUsed := make(map[string]int)
	powerUsed := make(map[string]int)
	upsUsed := make(map[string]int)
	// 每柜占用标记：occupied[rackIdx][u-1]
	occupied := make(map[int][]bool)

	for _, p := range placements {
		req, ok := reqByID[p.RequestID]
		if !ok {
			return fmt.Errorf("placement 引用了不存在的请求 %q", p.RequestID)
		}
		if accepted[p.RequestID] {
			return fmt.Errorf("请求 %q 被重复接纳", p.RequestID)
		}
		ri, ok := rackIdx[p.RackID]
		if !ok {
			return fmt.Errorf("placement 引用了不存在的机柜 %q", p.RackID)
		}
		rack := snapshot.Racks[ri]

		end := p.StartU + req.HeightU - 1
		if p.StartU < 1 || end > rack.HeightU {
			return fmt.Errorf("请求 %q 的占位 [%d,%d] 超出机柜 %q 高度范围",
				p.RequestID, p.StartU, end, p.RackID)
		}
		if !fitsInOneInterval(rack.Free, p.StartU, end) {
			return fmt.Errorf("请求 %q 的占位 [%d,%d] 未完整落在机柜 %q 的任一空闲区间内",
				p.RequestID, p.StartU, end, p.RackID)
		}

		occ := occupied[ri]
		if occ == nil {
			occ = make([]bool, rack.HeightU)
			occupied[ri] = occ
		}
		for u := p.StartU; u <= end; u++ {
			if occ[u-1] {
				return fmt.Errorf("请求 %q 与他人在机柜 %q 的 U%d 重叠",
					p.RequestID, p.RackID, u)
			}
			occ[u-1] = true
		}

		accepted[p.RequestID] = true
		weightUsed[p.RackID] += req.Weight
		powerUsed[p.RackID] += req.Power
		upsUsed[rack.UPS] += req.Power
	}

	for rackID, w := range weightUsed {
		if w > snapshot.Racks[rackIdx[rackID]].WeightRemainLb {
			return fmt.Errorf("机柜 %q 新设备重量 %d 超出剩余承重 %d",
				rackID, w, snapshot.Racks[rackIdx[rackID]].WeightRemainLb)
		}
	}
	for rackID, pw := range powerUsed {
		if pw > snapshot.Racks[rackIdx[rackID]].PowerRemainW {
			return fmt.Errorf("机柜 %q 新设备功率 %d 超出剩余功率预算 %d",
				rackID, pw, snapshot.Racks[rackIdx[rackID]].PowerRemainW)
		}
	}
	for upsID, pw := range upsUsed {
		if pw > upsRemain[upsID] {
			return fmt.Errorf("UPS %q 新设备功率 %d 超出共享余量 %d",
				upsID, pw, upsRemain[upsID])
		}
	}
	return nil
}

// fitsInOneInterval 报告闭区间 [start,end] 是否完整包含于某一段空闲区间。
func fitsInOneInterval(free []Interval, start, end int) bool {
	for _, iv := range free {
		if iv.Start <= start && end <= iv.End {
			return true
		}
	}
	return false
}
