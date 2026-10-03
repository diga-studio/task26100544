# rackplan

纯 Go 标准库实现的 DCIM 机柜上架规划库。输入同一房间的可信容量快照与一批
新设备请求，求出**接纳设备数量全局最大**的上架方案；现有资产不可挪动，
快照在求解期间不变。

## 能力

- `rackplan.Plan(snapshot, requests)`：返回接纳设备的标识、目标机柜与起始 U 位。
  未出现的请求视为未接纳；同样最优的方案允许任选。
- `rackplan.Validate(snapshot, requests, placements)`：独立检查方案是否满足
  全部安装约束（U 位范围、空闲区间包含、占位不重叠、机柜承重/功率、
  UPS 共享功率），不证明最优性，也不调用 `Plan`。

## 约束语义

- 机柜高度 12~52U，U 位从 1 编号；空闲区间为已排序、不重叠、相邻已合并的闭区间。
- 每台设备完整落在某机柜的一段空闲区间内，设备间不得占用相同 U 位。
- 新设备重量之和不超过目标机柜剩余承重；功率同时受机柜预算与所属 UPS
  共享预算约束（UPS 余量已扣除所有现有负载，不能由机柜余量反推）。
- 恰好用尽预算合法；设备不可拆分，不相邻空闲段不可拼合。
- 求解为精确分支限界：不输出贪心/近似/超时截断结果；各机柜不会脱离
  共享 UPS 预算独立求解。

## 规模与性能

支持最多 16 请求、8 机柜、24 段空闲区间、4 个 UPS。满规模固定基准
（含重复规格、且无法全部接纳，最优为 15/16）在本机（Apple M1 Max）
单次求解约 5~8ms，远低于 10 秒上限。

## 调用示例

```go
snapshot := rackplan.Snapshot{
    Racks: []rackplan.Rack{{
        ID: "RACK-A", HeightU: 42,
        Free:              []rackplan.Interval{{StartU: 5, EndU: 12}},
        RemainingWeightLB: 800, RemainingPowerW: 2400, UPSID: "UPS-1",
    }},
    UPSs: []rackplan.UPS{{ID: "UPS-1", RemainingPowerW: 3000}},
}
requests := []rackplan.Request{
    {ID: "srv-01", HeightU: 2, WeightLB: 80, PowerW: 450},
}
placements := rackplan.Plan(snapshot, requests)
err := rackplan.Validate(snapshot, requests, placements)
```

完整可运行示例见 `rackplan/example_test.go`。

## 运行测试与基准

```sh
# 全部测试（场景用例 + 300 组随机实例穷举核对 + Validate 拒绝用例 + 满规模计时）
go test ./... -v

# 仅满规模用例（打印本机求解耗时）
go test ./rackplan/ -run TestFullScale -v

# 满规模基准（不含编译）
go test ./rackplan/ -run '^$' -bench BenchmarkFullScale -benchtime 10x
```
