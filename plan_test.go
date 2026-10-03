package dcim

import "testing"

// TestContiguousSpaceRequired 总 U 数、重量、功率均有余量，
// 但空闲空间被切成两段 2U，3U 设备无处可放。
func TestContiguousSpaceRequired(t *testing.T) {
	snap := Snapshot{
		Racks: []Rack{{
			ID: "R1", HeightU: 12, UPS: "U1",
			Free:           []Interval{{Start: 1, End: 2}, {Start: 5, End: 6}},
			WeightRemainLb: 2500, PowerRemainW: 10000,
		}},
		UPS: []UPS{{ID: "U1", PowerRemainW: 10000}},
	}
	reqs := []Request{{ID: "big", HeightU: 3, Weight: 100, Power: 100}}
	got := Plan(snap, reqs)
	if len(got) != 0 {
		t.Fatalf("3U 设备不应被接纳，得到 %v", got)
	}
	if err := Validate(snap, reqs, got); err != nil {
		t.Fatalf("空方案应合法: %v", err)
	}

	// 换成两台 2U 设备则都能装下，证明余量本身足够。
	reqs = []Request{
		{ID: "a", HeightU: 2, Weight: 100, Power: 100},
		{ID: "b", HeightU: 2, Weight: 100, Power: 100},
	}
	got = Plan(snap, reqs)
	if len(got) != 2 {
		t.Fatalf("两台 2U 设备应都被接纳，得到 %v", got)
	}
	if err := Validate(snap, reqs, got); err != nil {
		t.Fatalf("方案应合法: %v", err)
	}
}

// TestSharedUPSBudget 每台设备在各自机柜单独合法，
// 合并后超过共享 UPS 预算，只能接纳其中一台。
func TestSharedUPSBudget(t *testing.T) {
	snap := Snapshot{
		Racks: []Rack{
			{ID: "R1", HeightU: 12, UPS: "U1",
				Free: []Interval{{Start: 1, End: 12}}, WeightRemainLb: 2500, PowerRemainW: 5000},
			{ID: "R2", HeightU: 12, UPS: "U1",
				Free: []Interval{{Start: 1, End: 12}}, WeightRemainLb: 2500, PowerRemainW: 5000},
		},
		UPS: []UPS{{ID: "U1", PowerRemainW: 3000}},
	}
	reqs := []Request{
		{ID: "a", HeightU: 2, Weight: 100, Power: 2000},
		{ID: "b", HeightU: 2, Weight: 100, Power: 2000},
	}
	got := Plan(snap, reqs)
	if len(got) != 1 {
		t.Fatalf("共享 UPS 预算 3000W 只够一台 2000W 设备，得到 %d 台", len(got))
	}
	if err := Validate(snap, reqs, got); err != nil {
		t.Fatalf("方案应合法: %v", err)
	}
}

// TestPreferManySmall 先接纳一台大设备只能装 1 台，
// 改选小设备可以装多台，Plan 必须选后者。
func TestPreferManySmall(t *testing.T) {
	snap := Snapshot{
		Racks: []Rack{{
			ID: "R1", HeightU: 12, UPS: "U1",
			Free:           []Interval{{Start: 1, End: 4}},
			WeightRemainLb: 2500, PowerRemainW: 10000,
		}},
		UPS: []UPS{{ID: "U1", PowerRemainW: 10000}},
	}
	reqs := []Request{
		{ID: "big", HeightU: 4, Weight: 100, Power: 100},
		{ID: "s1", HeightU: 1, Weight: 100, Power: 100},
		{ID: "s2", HeightU: 1, Weight: 100, Power: 100},
		{ID: "s3", HeightU: 1, Weight: 100, Power: 100},
	}
	got := Plan(snap, reqs)
	if len(got) != 3 {
		t.Fatalf("应接纳 3 台 1U 设备而非 1 台 4U 设备，得到 %v", got)
	}
	for _, p := range got {
		if p.RequestID == "big" {
			t.Fatalf("最优方案不应接纳 big: %v", got)
		}
	}
	if err := Validate(snap, reqs, got); err != nil {
		t.Fatalf("方案应合法: %v", err)
	}
}
