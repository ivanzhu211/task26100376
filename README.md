# weighplan

配方批次称量规划库（纯 Go，无外部依赖，配方与称量单仅保存在进程内存中）。

给定本批投料总重 `M`（整数克，库不会修改）和每种原料的「目标占比 / 占比允差 /
称量步长」，生成一份可直接执行的称量单：

- 包含全部原料，每项投料量是其称量步长的正整数倍；
- 所有投料量之和恰好等于 `M`；
- 每项实际占比与目标占比之差不超过该原料允差（边界计入）；
- 在所有合法称量单中总偏差最小；总偏差并列时，按原料编号升序排列的
  投料量序列字典序最小（与输入顺序无关）；
- 无合法解时明确返回 `ErrNoSolution`，不会用近似解代替。

允差判断、最优值比较与并列判定全部使用整数精确运算，无浮点误差。

## 单位约定

- 目标占比、允差：万分之一（全部原料目标占比之和必须为 10000，允差 0–50）。
- 投料量、总重：整数克。
- `Plan.TotalDeviation`：总偏差，单位为 1/10000 克，即
  `sum |Amount_i*10000 - M*TargetPpm_i|`。

## 使用示例

```go
package main

import (
	"errors"
	"fmt"

	"weighplan"
)

func main() {
	ingredients := []weighplan.Ingredient{
		{ID: 1, TargetPpm: 5000, TolerancePpm: 50, StepGrams: 4},
		{ID: 2, TargetPpm: 5000, TolerancePpm: 50, StepGrams: 6},
	}
	plan, err := weighplan.PlanBatch(1000, ingredients)
	if errors.Is(err, weighplan.ErrNoSolution) {
		fmt.Println("无可行称量单")
		return
	}
	if err != nil {
		panic(err) // 参数非法
	}
	for _, it := range plan.Items { // 按原料编号升序
		fmt.Printf("原料 %d: %d g\n", it.ID, it.Amount)
	}
	fmt.Printf("总偏差: %d/10000 g\n", plan.TotalDeviation)
}
```

## API

- `func PlanBatch(M int64, ingredients []Ingredient) (Plan, error)`
  - `M <= 0`、原料为空、编号非正或重复、目标占比非正或总和不为 10000、
    允差越界、步长非正：返回描述性参数错误。
  - 无可行解：返回 `ErrNoSolution`（可用 `errors.Is` 判断）。
  - 不修改输入切片，输入顺序不影响结果。
- `type Ingredient`：`ID`、`TargetPpm`、`TolerancePpm`、`StepGrams`。
- `type Plan`：`Items`（按编号升序）、`TotalDeviation`（1/10000 克）。

## 算法说明

设 `base_i` 为原料 `i` 允差区间内最小的合法投料量，则任意合法解可写成
`amount_i = base_i + m_i*step_i` 且 `sum m_i*step_i = M - sum base_i`。
在此有界整数问题上做后缀动态规划求精确最小总偏差，再按编号升序贪心
重建字典序最小的投料序列。复杂度约为 `O(n * R * maxCap)`，其中
`R = M - sum base_i`（由各原料允差下限决定），`maxCap` 为单原料允差区间
内可选步数的上界。

## 测试

```sh
go test ./...
```

覆盖：逐项就近取整不能闭合但存在合法组合、允差边界上多个最优方案的
字典序选择、无解判定、输入顺序无关性、参数校验，以及 300 组随机小规模
用例与穷举求解器的全量交叉验证（总重、步长、允差、全局最优、并列规则）。
