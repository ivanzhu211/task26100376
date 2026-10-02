# weighplan

配方批次称量规划库（纯进程内存，无外部依赖）。上层生产计划给定本批投料总重
M（整数克）与配方中各原料的目标占比、占比允差和称量步长，本库生成可直接
执行、总重恰好等于 M 且最贴近标准配方的称量单。

## 安装

```go
import "weighplan"
```

## 使用

```go
ings := []weighplan.Ingredient{
    {ID: 1, Target: 5000, Tolerance: 50, Step: 6}, // 目标 50%，允差 ±0.5%，步长 6 克
    {ID: 2, Target: 5000, Tolerance: 50, Step: 4},
}
ticket, err := weighplan.Plan(1000, ings)
switch {
case errors.Is(err, weighplan.ErrNoSolution):
    // 不存在满足全部约束的称量单
case errors.Is(err, weighplan.ErrInvalidInput):
    // 输入违反约定（编号、占比、允差、步长）
case err != nil:
    // 不可达
default:
    for _, item := range ticket.Items { // 按原料编号升序
        fmt.Printf("原料 %d: 投料 %d 克\n", item.ID, item.Amount)
    }
}
```

## 语义约定

- `Ingredient.Target` / `Tolerance` 以万分之一为单位：Target 为正整数且全部
  原料之和必须为 10000；Tolerance 取值 0 至 50（边界计入允许范围）。
- `Plan` 不改变 M：称量单各项之和恰好等于 M，每项为其步长的正整数倍，
  且包含全部原料。
- 合法称量单中总偏差最小：单项偏差为实际投料克数与理论投料克数
  （M × 目标占比，不预先取整）之差的绝对值，总偏差为全部单项偏差之和。
- 总偏差相同时，按原料编号升序的投料量序列取字典序最小者；
  结果与输入原料的排列顺序无关。
- 允差判断、最优值比较与并列判定均为精确整数语义（偏差放大 10000 倍
  比较），无浮点误差。`Ticket.DeviationNum` 为总偏差分子，
  总偏差 = `DeviationNum / 10000` 克。
- 要求 `M * 10000` 不溢出 int64。

## 测试

```sh
go test ./...
```

覆盖：逐项就近取整不能闭合但存在合法组合、允差边界上多个最优方案的
字典序选择、无解与非法输入、输入顺序不变性，以及 3000 组随机用例与
暴力枚举的全局最优性对照。
