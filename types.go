// Package weighplan 提供配方批次称量规划能力。
//
// 上层生产计划给定本批投料总重 M（整数克）与配方中各原料的
// 目标占比、占比允差和称量步长，本包在进程内存中计算一份可直接执行、
// 总重恰好等于 M、各项均为各自步长正整数倍、占比落入允差范围，
// 且总偏差最小的称量单；总偏差并列时按原料编号升序取投料量字典序最小者。
package weighplan

import "errors"

// ErrNoSolution 表示不存在满足全部约束的合法称量单。
var ErrNoSolution = errors.New("weighplan: no feasible weighing plan")

// Ingredient 描述一种原料的称量约束。
//
// TargetPpm 为目标占比，单位万分之一，正整数，全部原料之和必须为 10000。
// TolerancePpm 为占比允差，单位万分之一，取值 0 至 50（含边界）。
// StepGrams 为称量步长，单位克，正整数；投料量必须为其正整数倍。
type Ingredient struct {
	ID           int64
	TargetPpm    int64
	TolerancePpm int64
	StepGrams    int64
}

// Item 是称量单中的一项：原料编号与投料量（克）。
type Item struct {
	ID     int64
	Amount int64
}

// Plan 为一份合法且最优的称量单。
//
// Items 按原料编号升序排列，Amount 之和恰好等于 M。
// TotalDeviation 为总偏差，为保持精确数值语义，以 1/10000 克为单位：
// 即 sum |Amount_i*10000 - M*TargetPpm_i|。
type Plan struct {
	Items          []Item
	TotalDeviation int64
}
