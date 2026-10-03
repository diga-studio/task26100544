package dcim

import (
	"strings"
	"testing"
)

func validateTestSnapshot() Snapshot {
	return Snapshot{
		Racks: []Rack{
			{ID: "R1", HeightU: 12, UPS: "U1",
				Free: []Interval{{Start: 1, End: 6}}, WeightRemainLb: 500, PowerRemainW: 2000},
			{ID: "R2", HeightU: 12, UPS: "U1",
				Free: []Interval{{Start: 1, End: 6}}, WeightRemainLb: 500, PowerRemainW: 2000},
		},
		UPS: []UPS{{ID: "U1", PowerRemainW: 2500}},
	}
}

func validateTestRequests() []Request {
	return []Request{
		{ID: "a", HeightU: 2, Weight: 100, Power: 1000},
		{ID: "b", HeightU: 2, Weight: 100, Power: 1000},
	}
}

func TestValidateAcceptsLegal(t *testing.T) {
	snap := validateTestSnapshot()
	reqs := validateTestRequests()
	plan := []Placement{
		{RequestID: "a", RackID: "R1", StartU: 1},
		{RequestID: "b", RackID: "R2", StartU: 1},
	}
	if err := Validate(snap, reqs, plan); err != nil {
		t.Fatalf("合法方案被拒绝: %v", err)
	}
	// 空方案合法
	if err := Validate(snap, reqs, nil); err != nil {
		t.Fatalf("空方案被拒绝: %v", err)
	}
}

func TestValidateRejects(t *testing.T) {
	snap := validateTestSnapshot()
	reqs := validateTestRequests()
	cases := []struct {
		name string
		plan []Placement
		want string
	}{
		{
			name: "重叠占位",
			plan: []Placement{
				{RequestID: "a", RackID: "R1", StartU: 1},
				{RequestID: "b", RackID: "R1", StartU: 2},
			},
			want: "重叠",
		},
		{
			name: "共享UPS超额",
			plan: []Placement{
				{RequestID: "a", RackID: "R1", StartU: 1},
				{RequestID: "b", RackID: "R2", StartU: 1},
			},
			want: "UPS",
		},
		{
			name: "跨出空闲区间",
			plan: []Placement{
				{RequestID: "a", RackID: "R1", StartU: 6}, // 占 [6,7]，7 不在 [1,6]
			},
			want: "空闲区间",
		},
		{
			name: "重复接纳",
			plan: []Placement{
				{RequestID: "a", RackID: "R1", StartU: 1},
				{RequestID: "a", RackID: "R2", StartU: 1},
			},
			want: "重复",
		},
		{
			name: "未知请求",
			plan: []Placement{{RequestID: "ghost", RackID: "R1", StartU: 1}},
			want: "不存在",
		},
		{
			name: "未知机柜",
			plan: []Placement{{RequestID: "a", RackID: "RX", StartU: 1}},
			want: "不存在",
		},
	}
	for _, tc := range cases {
		// 共享UPS超额用例：两台 1000W 合计 2000W 未超 2500W，
		// 需要把 UPS 余量调小来构造超额。
		s := snap
		if tc.name == "共享UPS超额" {
			s.UPS = []UPS{{ID: "U1", PowerRemainW: 1500}}
		}
		err := Validate(s, reqs, tc.plan)
		if err == nil {
			t.Fatalf("%s: 应被拒绝却通过", tc.name)
		}
		if !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("%s: 错误信息 %q 不含 %q", tc.name, err.Error(), tc.want)
		}
	}
}

// TestValidateRejectsRackBudgets 机柜级重量/功率超额也要拒绝。
func TestValidateRejectsRackBudgets(t *testing.T) {
	snap := validateTestSnapshot()
	reqs := []Request{
		{ID: "heavy", HeightU: 1, Weight: 600, Power: 100},
		{ID: "hot", HeightU: 1, Weight: 100, Power: 3000},
	}
	if err := Validate(snap, reqs, []Placement{{RequestID: "heavy", RackID: "R1", StartU: 1}}); err == nil {
		t.Fatal("机柜重量超额应被拒绝")
	}
	snap.UPS = []UPS{{ID: "U1", PowerRemainW: 9000}}
	if err := Validate(snap, reqs, []Placement{{RequestID: "hot", RackID: "R1", StartU: 1}}); err == nil {
		t.Fatal("机柜功率超额应被拒绝")
	}
}
