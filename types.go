// Package dcim 提供数据中心机柜上架规划能力：
// 在一批新设备请求中选出全局最大可接纳子集，并给出每台设备的
// 目标机柜与起始 U 位。现有资产不可挪动，快照在求解期间不变。
package dcim

// Interval 是一段空闲 U 位的闭区间，U 位从 1 编号。
type Interval struct {
	Start int // 起始 U 位（含）
	End   int // 结束 U 位（含）
}

// Len 返回区间包含的 U 位数。
func (iv Interval) Len() int { return iv.End - iv.Start + 1 }

// Rack 描述一个机柜的剩余容量快照。
type Rack struct {
	ID             string
	HeightU        int        // 机柜总高度，12 至 52U
	UPS            string     // 所属 UPS 标识
	Free           []Interval // 空闲闭区间，已排序、不重叠、相邻已合并
	WeightRemainLb int        // 剩余承重（磅），已扣除现有负载
	PowerRemainW   int        // 剩余功率预算（W），已扣除现有负载
}

// UPS 描述一个 UPS 的共享剩余功率预算。
// 已扣除所有现有负载（含未列入本次规划的负载），
// 不能由机柜余量反推。
type UPS struct {
	ID           string
	PowerRemainW int
}

// Snapshot 是同一房间的可信容量快照，求解期间不变。
type Snapshot struct {
	Racks []Rack
	UPS   []UPS
}

// Request 是一台待上架新设备。各字段均为正整数。
type Request struct {
	ID      string
	HeightU int // 连续 U 高度
	Weight  int // 磅
	Power   int // W
}

// Placement 表示一台设备被接纳：放入 RackID 机柜，
// 占据 [StartU, StartU+HeightU-1] 的连续 U 位。
type Placement struct {
	RequestID string
	RackID    string
	StartU    int
}
