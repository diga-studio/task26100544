# dcim — DCIM 机柜上架规划库

纯 Go 标准库实现。对一批新设备的上架请求，求出**全局最大可接纳子集**，
并给出每台设备的目标机柜与起始 U 位。现有资产不可挪动，
输入为同一房间的可信容量快照，求解期间不变。

## 约束语义

- 设备不可拆分，必须完整落在某机柜的一段空闲闭区间内，设备间不占相同 U 位；
- 每柜新设备重量/功率不超过机柜剩余预算（重量已按 2500 磅上限扣除现有负载）；
- 同一 UPS 下所有机柜的新设备功率之和不超过该 UPS 的共享剩余功率
  （UPS 余量独立给出，不能由机柜余量反推）；
- 恰好用尽预算合法。

## 求解方法

`Plan` 使用单棵分支限界搜索树（非贪心、非近似、无超时截断）：
设备按高度降序，逐台决定“放入某段空闲区间 / 拒绝”，配合

- 聚合资源上界剪枝（剩余 U 数 / 承重 / 机柜功率 / UPS 功率分别估计还能装几台）；
- 同柜等长区间去重、相同规格设备的放置单调性与拒绝后缀规则，消除对称分支；
- 多种排序的首次适应贪心先产出强 incumbent，加速最优性证明。

`Validate` 独立检查方案是否满足全部安装约束，不证明最优性，也不调用 `Plan`。

## 调用示例

```go
snap := dcim.Snapshot{
    Racks: []dcim.Rack{
        {ID: "R1", HeightU: 42, UPS: "UPS-A",
            Free: []dcim.Interval{{Start: 1, End: 10}}, WeightRemainLb: 2000, PowerRemainW: 4000},
    },
    UPS: []dcim.UPS{{ID: "UPS-A", PowerRemainW: 5000}},
}
reqs := []dcim.Request{{ID: "web-1", HeightU: 2, Weight: 200, Power: 1500}}

plan := dcim.Plan(snap, reqs)          // []dcim.Placement：接纳的设备、机柜、起始 U 位
err  := dcim.Validate(snap, reqs, plan) // nil 表示方案合法
```

完整可运行示例见 `example_test.go`（`ExamplePlan`）。

## 运行测试与基准

```sh
go test ./...        # 全部单元测试（含穷举交叉核对、Validate 非法方案拒绝）
go test -v -run TestFullScale .   # 满规模固定输入：16 请求 / 8 机柜 / 24 区间 / 4 UPS
go test -run '^$' -bench BenchmarkPlanFullScale .  # 满规模基准
```

本机（Apple M1 Max）满规模单次求解约 37µs，远低于 10 秒上限。
基准输入中 16 台设备总功率 6800W 远超 UPS 合计 4100W，不可能全部接纳，
最优接纳数为 12。
