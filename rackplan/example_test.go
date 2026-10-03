package rackplan_test

import (
	"fmt"

	"rackplan/rackplan"
)

// Example 演示 Plan 与 Validate 的典型用法。
func Example() {
	snapshot := rackplan.Snapshot{
		Racks: []rackplan.Rack{
			{
				ID:      "RACK-A",
				HeightU: 42,
				Free: []rackplan.Interval{
					{StartU: 5, EndU: 12},
					{StartU: 20, EndU: 30},
				},
				RemainingWeightLB: 800,
				RemainingPowerW:   2400,
				UPSID:             "UPS-1",
			},
			{
				ID:                "RACK-B",
				HeightU:           42,
				Free:              []rackplan.Interval{{StartU: 1, EndU: 10}},
				RemainingWeightLB: 500,
				RemainingPowerW:   1500,
				UPSID:             "UPS-1",
			},
		},
		UPSs: []rackplan.UPS{
			{ID: "UPS-1", RemainingPowerW: 3000},
		},
	}
	requests := []rackplan.Request{
		{ID: "srv-01", HeightU: 2, WeightLB: 80, PowerW: 450},
		{ID: "srv-02", HeightU: 4, WeightLB: 150, PowerW: 800},
		{ID: "srv-03", HeightU: 1, WeightLB: 30, PowerW: 200},
	}

	placements := rackplan.Plan(snapshot, requests)
	for _, p := range placements {
		fmt.Printf("%s -> 机柜 %s, 起始 U%d\n", p.RequestID, p.RackID, p.StartU)
	}
	fmt.Printf("接纳 %d/%d 台\n", len(placements), len(requests))

	if err := rackplan.Validate(snapshot, requests, placements); err != nil {
		fmt.Println("方案非法:", err)
	} else {
		fmt.Println("方案通过校验")
	}
	// Output:
	// srv-02 -> 机柜 RACK-A, 起始 U5
	// srv-01 -> 机柜 RACK-A, 起始 U9
	// srv-03 -> 机柜 RACK-A, 起始 U11
	// 接纳 3/3 台
	// 方案通过校验
}
