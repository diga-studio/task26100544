// Package rackplan 提供 DCIM 机柜上架规划能力：
// 在给定房间容量快照与一批新设备请求时，求出可接纳设备数量的
// 全局最大值及对应上架方案，并可独立校验任意方案的合法性。
package rackplan

// Interval 表示一段空闲 U 位的闭区间（含端点），U 位从 1 编号。
type Interval struct {
	StartU int
	EndU   int
}

// Rack 表示一个机柜的容量快照。
type Rack struct {
	ID                string
	HeightU           int        // 机柜总高度，12 至 52U
	Free              []Interval // 空闲闭区间，已排序、不重叠、相邻已合并
	RemainingWeightLB int        // 剩余承重（磅），已扣除现有负载
	RemainingPowerW   int        // 机柜剩余功率预算（W）
	UPSID             string     // 所属 UPS 标识
}

// UPS 表示一个 UPS 的共享功率预算，已扣除所有现有负载。
type UPS struct {
	ID              string
	RemainingPowerW int
}

// Snapshot 是同一房间的可信容量快照，求解期间不变。
type Snapshot struct {
	Racks []Rack
	UPSs  []UPS
}

// Request 表示一台待上架的新设备。
type Request struct {
	ID       string
	HeightU  int // 连续 U 高度，正整数
	WeightLB int // 重量（磅），正整数
	PowerW   int // 功率（W），正整数
}

// Placement 表示一台设备的上架结果：目标机柜与起始 U 位。
type Placement struct {
	RequestID string
	RackID    string
	StartU    int
}
