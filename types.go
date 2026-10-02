// Package weighplan 提供配方批次称量规划能力。
//
// 上层生产计划给定本批投料总重 M（整数克）与配方中各原料的
// 目标占比、占比允差和称量步长，本包在进程内存中生成可直接执行、
// 总重恰好等于 M 且总偏差最小的称量单。
package weighplan

import "errors"

// ErrNoSolution 表示在给定约束下不存在合法称量单。
var ErrNoSolution = errors.New("weighplan: no feasible weighing ticket")

// ErrInvalidInput 表示输入参数不合法（编号、占比、允差或步长违反约定）。
var ErrInvalidInput = errors.New("weighplan: invalid input")

// Ingredient 描述一种原料的称量约束。
//
// Target 与 Tolerance 均以万分之一为单位：
// Target 为正整数且全部原料之和为 10000；Tolerance 取值 0 至 50。
type Ingredient struct {
	ID        int   // 唯一正整数编号
	Target    int   // 目标占比，万分之一
	Tolerance int   // 占比允差，万分之一，0..50
	Step      int64 // 称量步长，正整数克
}

// WeighItem 是称量单中的一项：某原料的实际投料克数。
type WeighItem struct {
	ID     int   // 原料编号
	Amount int64 // 投料量，克，为其步长的正整数倍
}

// Ticket 是一份可执行的称量单。
type Ticket struct {
	Items []WeighItem // 按原料编号从小到大排列
	// DeviationNum 是总偏差的精确分子：总偏差 = DeviationNum / 10000 克。
	// 即 sum(|Amount_i * 10000 - M * Target_i|)，避免浮点误差。
	DeviationNum int64
}

// Total 返回称量单的总投料克数，恒等于调用时的 M。
func (t *Ticket) Total() int64 {
	var sum int64
	for _, it := range t.Items {
		sum += it.Amount
	}
	return sum
}
