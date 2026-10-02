package weighplan

import (
	"math"
	"sort"
)

const infinity = math.MaxInt64 / 4

// solve 在通过校验的输入上求解。
//
// 设 base_i 为原料 i 允差区间内最小的合法投料量（步长的正整数倍），
// 则任意合法解可写成 amount_i = base_i + m_i*step_i（0 <= m_i <= cap_i），
// 且 sum m_i*step_i = M - sum base_i = R。
// 用后缀 DP 精确求最小总偏差，再按编号升序贪心重建字典序最小的投料序列。
func solve(M int64, ingredients []Ingredient) (Plan, error) {
	sorted := make([]Ingredient, len(ingredients))
	copy(sorted, ingredients)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].ID < sorted[j].ID })

	n := len(sorted)
	base := make([]int64, n)
	step := make([]int64, n)
	cap := make([]int64, n)
	theory := make([]int64, n) // M*TargetPpm，理论投料量的 10000 倍

	var baseSum int64
	for i, ing := range sorted {
		lo, hi := bounds(M, ing)
		if lo < ing.StepGrams {
			lo = ing.StepGrams // 投料量必须为步长的正整数倍
		}
		b := ceilDiv(lo, ing.StepGrams) * ing.StepGrams
		if b > hi {
			return Plan{}, ErrNoSolution
		}
		base[i] = b
		step[i] = ing.StepGrams
		cap[i] = (hi - b) / ing.StepGrams
		theory[i] = M * ing.TargetPpm
		baseSum += b
	}

	R := M - baseSum
	if R < 0 {
		return Plan{}, ErrNoSolution
	}

	dev := func(i int, m int64) int64 {
		amount := base[i] + m*step[i]
		d := amount*10000 - theory[i]
		if d < 0 {
			return -d
		}
		return d
	}

	// suf[i][s]：原料 i..n-1 贡献额外重量 s 时的最小总偏差，不可达为 infinity。
	suf := make([][]int64, n+1)
	for i := range suf {
		suf[i] = make([]int64, R+1)
		for s := range suf[i] {
			suf[i][s] = infinity
		}
	}
	suf[n][0] = 0
	for i := n - 1; i >= 0; i-- {
		for s := int64(0); s <= R; s++ {
			best := int64(infinity)
			for m := int64(0); m <= cap[i] && m*step[i] <= s; m++ {
				rest := suf[i+1][s-m*step[i]]
				if rest >= infinity {
					continue
				}
				if v := dev(i, m) + rest; v < best {
					best = v
				}
			}
			suf[i][s] = best
		}
	}

	if suf[0][R] >= infinity {
		return Plan{}, ErrNoSolution
	}

	// 按编号升序逐个取可行的最小 m_i，即投料量字典序最小的最优解。
	items := make([]Item, n)
	remaining := R
	for i := 0; i < n; i++ {
		for m := int64(0); m <= cap[i] && m*step[i] <= remaining; m++ {
			if dev(i, m)+suf[i+1][remaining-m*step[i]] == suf[i][remaining] {
				items[i] = Item{ID: sorted[i].ID, Amount: base[i] + m*step[i]}
				remaining -= m * step[i]
				break
			}
		}
	}

	return Plan{Items: items, TotalDeviation: suf[0][R]}, nil
}
