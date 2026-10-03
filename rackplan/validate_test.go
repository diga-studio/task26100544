package rackplan

import "testing"

func validateTestSnapshot() Snapshot {
	return Snapshot{
		Racks: []Rack{
			{ID: "R1", HeightU: 24, Free: []Interval{{1, 6}, {9, 12}}, RemainingWeightLB: 300, RemainingPowerW: 1000, UPSID: "U1"},
			{ID: "R2", HeightU: 24, Free: []Interval{{1, 6}}, RemainingWeightLB: 300, RemainingPowerW: 1000, UPSID: "U1"},
		},
		UPSs: []UPS{{ID: "U1", RemainingPowerW: 1200}},
	}
}

func validateTestRequests() []Request {
	return []Request{
		{ID: "a", HeightU: 2, WeightLB: 100, PowerW: 500},
		{ID: "b", HeightU: 2, WeightLB: 100, PowerW: 500},
	}
}

func TestValidateAcceptsLegalPlan(t *testing.T) {
	snap := validateTestSnapshot()
	reqs := validateTestRequests()
	placements := []Placement{
		{RequestID: "a", RackID: "R1", StartU: 1},
		{RequestID: "b", RackID: "R2", StartU: 1},
	}
	if err := Validate(snap, reqs, placements); err != nil {
		t.Fatalf("合法方案应通过校验: %v", err)
	}
	// 空方案（全部未接纳）也合法。
	if err := Validate(snap, reqs, nil); err != nil {
		t.Fatalf("空方案应通过校验: %v", err)
	}
}

func TestValidateRejectsOverlap(t *testing.T) {
	snap := validateTestSnapshot()
	reqs := validateTestRequests()
	placements := []Placement{
		{RequestID: "a", RackID: "R1", StartU: 1},
		{RequestID: "b", RackID: "R1", StartU: 2}, // 与 a 占用 [1,2] 重叠
	}
	if err := Validate(snap, reqs, placements); err == nil {
		t.Fatal("重叠占位的方案应被拒绝")
	}
}

func TestValidateRejectsSharedUPSOverload(t *testing.T) {
	snap := validateTestSnapshot()
	reqs := validateTestRequests()
	// 每台 500W，两柜机柜预算各自足够，但共享 UPS 只有 1200W 余量：
	// 先构造刚好合法（1000W），再超 1W 验证拒绝。
	placements := []Placement{
		{RequestID: "a", RackID: "R1", StartU: 1},
		{RequestID: "b", RackID: "R2", StartU: 1},
	}
	if err := Validate(snap, reqs, placements); err != nil {
		t.Fatalf("恰好用尽共享 UPS 预算应合法: %v", err)
	}
	snap.UPSs[0].RemainingPowerW = 999
	if err := Validate(snap, reqs, placements); err == nil {
		t.Fatal("共享 UPS 超额的方案应被拒绝")
	}
}

func TestValidateRejectsRackBudgetOverload(t *testing.T) {
	snap := validateTestSnapshot()
	reqs := validateTestRequests()
	snap.UPSs[0].RemainingPowerW = 5000
	snap.Racks[0].RemainingPowerW = 999 // 两台各 500W，机柜预算不够
	placements := []Placement{
		{RequestID: "a", RackID: "R1", StartU: 1},
		{RequestID: "b", RackID: "R1", StartU: 3},
	}
	if err := Validate(snap, reqs, placements); err == nil {
		t.Fatal("机柜功率超额的方案应被拒绝")
	}
	snap.Racks[0].RemainingPowerW = 1000
	snap.Racks[0].RemainingWeightLB = 199 // 两台各 100 磅，承重不够
	if err := Validate(snap, reqs, placements); err == nil {
		t.Fatal("机柜承重超额的方案应被拒绝")
	}
}

func TestValidateRejectsPlacementViolations(t *testing.T) {
	snap := validateTestSnapshot()
	reqs := validateTestRequests()
	cases := map[string][]Placement{
		"未知请求":     {{RequestID: "ghost", RackID: "R1", StartU: 1}},
		"未知机柜":     {{RequestID: "a", RackID: "R9", StartU: 1}},
		"重复接纳":     {{RequestID: "a", RackID: "R1", StartU: 1}, {RequestID: "a", RackID: "R2", StartU: 1}},
		"起始U位非法":   {{RequestID: "a", RackID: "R1", StartU: 0}},
		"超出机柜高度":   {{RequestID: "a", RackID: "R1", StartU: 24}},
		"跨空闲段":     {{RequestID: "a", RackID: "R1", StartU: 6}}, // [6,7] 跨越两段空闲
		"落在占用区":    {{RequestID: "a", RackID: "R1", StartU: 7}}, // [7,8] 不在任何空闲区间内
		"第二台落在占用区": {{RequestID: "a", RackID: "R1", StartU: 1}, {RequestID: "b", RackID: "R1", StartU: 7}},
	}
	for name, placements := range cases {
		if err := Validate(snap, reqs, placements); err == nil {
			t.Errorf("%s: 应被拒绝", name)
		}
	}
}
