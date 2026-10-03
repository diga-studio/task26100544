package dcim_test

import (
	"fmt"
	"sort"

	"dcim"
)

// ExamplePlan 演示：两台机柜共享一个 UPS，三台设备争夺功率预算，
// Plan 选出全局最大可接纳子集，Validate 独立校验方案合法性。
func ExamplePlan() {
	snap := dcim.Snapshot{
		Racks: []dcim.Rack{
			{ID: "R1", HeightU: 42, UPS: "UPS-A",
				Free: []dcim.Interval{{Start: 1, End: 10}}, WeightRemainLb: 2000, PowerRemainW: 4000},
			{ID: "R2", HeightU: 42, UPS: "UPS-A",
				Free: []dcim.Interval{{Start: 5, End: 12}}, WeightRemainLb: 1500, PowerRemainW: 4000},
		},
		UPS: []dcim.UPS{{ID: "UPS-A", PowerRemainW: 5000}},
	}
	requests := []dcim.Request{
		{ID: "web-1", HeightU: 2, Weight: 200, Power: 1500},
		{ID: "web-2", HeightU: 2, Weight: 200, Power: 1500},
		{ID: "db-1", HeightU: 4, Weight: 500, Power: 3000},
	}

	plan := dcim.Plan(snap, requests)
	sort.Slice(plan, func(i, j int) bool { return plan[i].RequestID < plan[j].RequestID })
	for _, p := range plan {
		fmt.Printf("%s -> 机柜 %s, 起始 U%d\n", p.RequestID, p.RackID, p.StartU)
	}

	if err := dcim.Validate(snap, requests, plan); err != nil {
		fmt.Println("方案非法:", err)
	} else {
		fmt.Println("方案合法")
	}
	// Output:
	// web-1 -> 机柜 R1, 起始 U1
	// web-2 -> 机柜 R1, 起始 U3
	// 方案合法
}
