package weighplan

import "fmt"

// Plan 根据投料总重 total（整数克）与配方原料列表生成最优称量单。
//
// 返回的称量单满足：包含全部原料；每项投料量为其步长的正整数倍；
// 总量恰好等于 total；每项实际占比与目标占比之差不超过其允差。
// 在所有合法称量单中总偏差最小；总偏差相同时按原料编号升序的
// 投料量序列字典序最小。无解时返回 ErrNoSolution；
// 输入不合法时返回包装了 ErrInvalidInput 的错误。
//
// 全部计算使用 int64 精确整数语义（偏差放大 10000 倍比较），
// 要求 total*10000 不溢出 int64。
func Plan(total int64, ingredients []Ingredient) (*Ticket, error) {
	if err := validate(total, ingredients); err != nil {
		return nil, err
	}

	sorted := make([]Ingredient, len(ingredients))
	copy(sorted, ingredients)
	sortByID(sorted)

	n := len(sorted)
	lo := make([]int64, n)
	hi := make([]int64, n)
	for i, ing := range sorted {
		l, h, ok := feasibleRange(total, ing)
		if !ok {
			return nil, ErrNoSolution
		}
		lo[i], hi[i] = l, h
	}

	// 后缀和用于剪枝：剩余原料的最小/最大可投总重。
	sufLo := make([]int64, n+1)
	sufHi := make([]int64, n+1)
	for i := n - 1; i >= 0; i-- {
		sufLo[i] = sufLo[i+1] + lo[i]
		sufHi[i] = sufHi[i+1] + hi[i]
	}
	if sufLo[0] > total || sufHi[0] < total {
		return nil, ErrNoSolution
	}

	type state struct {
		dev     int64   // 偏差分子：sum(|amount*10000 - total*target|)
		amounts []int64 // 已处理原料（按编号升序）的投料量
	}

	states := map[int64]state{0: {}}
	for i, ing := range sorted {
		theory := total * int64(ing.Target) // 理论投料量 * 10000
		next := make(map[int64]state, len(states))
		for w, s := range states {
			for a := lo[i]; a <= hi[i]; a += ing.Step {
				nw := w + a
				// 剪枝：加上剩余原料后必须仍能闭合到 total。
				if nw+sufLo[i+1] > total || nw+sufHi[i+1] < total {
					continue
				}
				nd := s.dev + abs64(a*10000-theory)
				cand := append(append([]int64{}, s.amounts...), a)
				cur, ok := next[nw]
				if !ok || nd < cur.dev || (nd == cur.dev && lessAmounts(cand, cur.amounts)) {
					next[nw] = state{dev: nd, amounts: cand}
				}
			}
		}
		states = next
	}

	best, ok := states[total]
	if !ok {
		return nil, ErrNoSolution
	}

	ticket := &Ticket{DeviationNum: best.dev}
	for i, ing := range sorted {
		ticket.Items = append(ticket.Items, WeighItem{ID: ing.ID, Amount: best.amounts[i]})
	}
	return ticket, nil
}

func validate(total int64, ingredients []Ingredient) error {
	if total <= 0 {
		return fmt.Errorf("%w: total must be positive, got %d", ErrInvalidInput, total)
	}
	if len(ingredients) == 0 {
		return fmt.Errorf("%w: no ingredients", ErrInvalidInput)
	}
	seen := make(map[int]struct{}, len(ingredients))
	sumTarget := 0
	for _, ing := range ingredients {
		if ing.ID <= 0 {
			return fmt.Errorf("%w: ingredient ID must be positive, got %d", ErrInvalidInput, ing.ID)
		}
		if _, dup := seen[ing.ID]; dup {
			return fmt.Errorf("%w: duplicate ingredient ID %d", ErrInvalidInput, ing.ID)
		}
		seen[ing.ID] = struct{}{}
		if ing.Target <= 0 {
			return fmt.Errorf("%w: ingredient %d target must be positive, got %d", ErrInvalidInput, ing.ID, ing.Target)
		}
		sumTarget += ing.Target
		if ing.Tolerance < 0 || ing.Tolerance > 50 {
			return fmt.Errorf("%w: ingredient %d tolerance must be in [0,50], got %d", ErrInvalidInput, ing.ID, ing.Tolerance)
		}
		if ing.Step <= 0 {
			return fmt.Errorf("%w: ingredient %d step must be positive, got %d", ErrInvalidInput, ing.ID, ing.Step)
		}
	}
	if sumTarget != 10000 {
		return fmt.Errorf("%w: targets must sum to 10000, got %d", ErrInvalidInput, sumTarget)
	}
	return nil
}

// feasibleRange 返回原料在允差窗口内、为步长正整数倍的最小/最大投料量。
func feasibleRange(total int64, ing Ingredient) (lo, hi int64, ok bool) {
	t := total * int64(ing.Target)
	tol := total * int64(ing.Tolerance)
	lower := ceilDiv(t-tol, 10000)
	upper := floorDiv(t+tol, 10000)
	if lower < ing.Step {
		lower = ing.Step // 投料量必须是步长的正整数倍
	}
	lo = ceilDiv(lower, ing.Step) * ing.Step
	hi = floorDiv(upper, ing.Step) * ing.Step
	return lo, hi, lo <= hi
}

func floorDiv(a, b int64) int64 {
	q := a / b
	if a%b != 0 && (a < 0) != (b < 0) {
		q--
	}
	return q
}

func ceilDiv(a, b int64) int64 {
	return -floorDiv(-a, b)
}

func abs64(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}

// lessAmounts 比较两个等长投料量序列的字典序。
func lessAmounts(a, b []int64) bool {
	for i := range a {
		if a[i] != b[i] {
			return a[i] < b[i]
		}
	}
	return false
}

func sortByID(ings []Ingredient) {
	for i := 1; i < len(ings); i++ {
		for j := i; j > 0 && ings[j].ID < ings[j-1].ID; j-- {
			ings[j], ings[j-1] = ings[j-1], ings[j]
		}
	}
}
