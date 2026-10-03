package rackplan

import "fmt"

// Validate 独立检查方案是否满足全部安装约束。
// 合法返回 nil，否则返回描述首个违规项的错误。
// 它不负责证明最优性，也不调用 Plan。
func Validate(snapshot Snapshot, requests []Request, placements []Placement) error {
	reqByID := make(map[string]Request, len(requests))
	for _, r := range requests {
		reqByID[r.ID] = r
	}
	rackByID := make(map[string]*Rack, len(snapshot.Racks))
	for i := range snapshot.Racks {
		rackByID[snapshot.Racks[i].ID] = &snapshot.Racks[i]
	}
	upsByID := make(map[string]*UPS, len(snapshot.UPSs))
	for i := range snapshot.UPSs {
		upsByID[snapshot.UPSs[i].ID] = &snapshot.UPSs[i]
	}

	seen := make(map[string]bool, len(placements))
	weightUsed := make(map[string]int, len(snapshot.Racks))
	rackPowerUsed := make(map[string]int, len(snapshot.Racks))
	upsPowerUsed := make(map[string]int, len(snapshot.UPSs))
	// 每个机柜内已占用的 U 位区间，用于重叠检查。
	occupied := make(map[string][]Interval, len(snapshot.Racks))

	for _, p := range placements {
		req, ok := reqByID[p.RequestID]
		if !ok {
			return fmt.Errorf("placement 引用了未知请求 %q", p.RequestID)
		}
		if seen[p.RequestID] {
			return fmt.Errorf("请求 %q 被重复接纳", p.RequestID)
		}
		seen[p.RequestID] = true

		rack, ok := rackByID[p.RackID]
		if !ok {
			return fmt.Errorf("请求 %q 的目标机柜 %q 不存在", p.RequestID, p.RackID)
		}
		if p.StartU < 1 || p.StartU+req.HeightU-1 > rack.HeightU {
			return fmt.Errorf("请求 %q 超岀机柜 %q 的 U 位范围", p.RequestID, p.RackID)
		}
		endU := p.StartU + req.HeightU - 1
		if !containedInFree(rack.Free, p.StartU, endU) {
			return fmt.Errorf("请求 %q 未完整落在机柜 %q 的任一空闲区间内", p.RequestID, p.RackID)
		}
		for _, occ := range occupied[rack.ID] {
			if p.StartU <= occ.EndU && occ.StartU <= endU {
				return fmt.Errorf("请求 %q 与机柜 %q 内 [%d,%d] 的设备占位重叠", p.RequestID, rack.ID, occ.StartU, occ.EndU)
			}
		}
		occupied[rack.ID] = append(occupied[rack.ID], Interval{StartU: p.StartU, EndU: endU})

		weightUsed[rack.ID] += req.WeightLB
		if weightUsed[rack.ID] > rack.RemainingWeightLB {
			return fmt.Errorf("机柜 %q 剩余承重不足", rack.ID)
		}
		rackPowerUsed[rack.ID] += req.PowerW
		if rackPowerUsed[rack.ID] > rack.RemainingPowerW {
			return fmt.Errorf("机柜 %q 剩余功率预算不足", rack.ID)
		}
		ups, ok := upsByID[rack.UPSID]
		if !ok {
			return fmt.Errorf("机柜 %q 引用了未知 UPS %q", rack.ID, rack.UPSID)
		}
		upsPowerUsed[ups.ID] += req.PowerW
		if upsPowerUsed[ups.ID] > ups.RemainingPowerW {
			return fmt.Errorf("UPS %q 共享功率预算不足", ups.ID)
		}
	}
	return nil
}

// containedInFree 判断闭区间 [startU, endU] 是否完整落在某个空闲区间内。
func containedInFree(free []Interval, startU, endU int) bool {
	for _, iv := range free {
		if iv.StartU <= startU && endU <= iv.EndU {
			return true
		}
	}
	return false
}
