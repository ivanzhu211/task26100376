package weighplan

import "fmt"

// PlanBatch 根据投料总重 M（克）与配方原料约束生成最优称量单。
//
// 无解时返回 ErrNoSolution；参数非法时返回描述性错误。
// 输入原料切片不会被修改，其排列顺序不影响结果。
func PlanBatch(M int64, ingredients []Ingredient) (Plan, error) {
	if err := validate(M, ingredients); err != nil {
		return Plan{}, err
	}
	return solve(M, ingredients)
}

func validate(M int64, ingredients []Ingredient) error {
	if M <= 0 {
		return fmt.Errorf("weighplan: total weight M must be positive, got %d", M)
	}
	if len(ingredients) == 0 {
		return fmt.Errorf("weighplan: at least one ingredient is required")
	}
	seen := make(map[int64]bool, len(ingredients))
	var targetSum int64
	for _, ing := range ingredients {
		if ing.ID <= 0 {
			return fmt.Errorf("weighplan: ingredient ID must be positive, got %d", ing.ID)
		}
		if seen[ing.ID] {
			return fmt.Errorf("weighplan: duplicate ingredient ID %d", ing.ID)
		}
		seen[ing.ID] = true
		if ing.TargetPpm <= 0 {
			return fmt.Errorf("weighplan: ingredient %d target must be positive, got %d", ing.ID, ing.TargetPpm)
		}
		targetSum += ing.TargetPpm
		if ing.TolerancePpm < 0 || ing.TolerancePpm > 50 {
			return fmt.Errorf("weighplan: ingredient %d tolerance must be in [0,50], got %d", ing.ID, ing.TolerancePpm)
		}
		if ing.StepGrams <= 0 {
			return fmt.Errorf("weighplan: ingredient %d step must be positive, got %d", ing.ID, ing.StepGrams)
		}
	}
	if targetSum != 10000 {
		return fmt.Errorf("weighplan: targets must sum to 10000, got %d", targetSum)
	}
	return nil
}

// bounds 为原料 i 计算允差约束下的投料量闭区间 [lo, hi]（克），
// 精确语义：|amount*10000 - M*target| <= M*tolerance。
func bounds(M int64, ing Ingredient) (lo, hi int64) {
	lower := M * (ing.TargetPpm - ing.TolerancePpm) // 可能为负
	upper := M * (ing.TargetPpm + ing.TolerancePpm)
	lo = ceilDiv(lower, 10000)
	hi = floorDiv(upper, 10000)
	return lo, hi
}

// ceilDiv 返回不小于 a/b 的最小整数，b 必须为正。
func ceilDiv(a, b int64) int64 {
	return -floorDiv(-a, b)
}

// floorDiv 返回不大于 a/b 的最大整数，b 必须为正。
func floorDiv(a, b int64) int64 {
	q := a / b
	if a%b != 0 && (a < 0) != (b < 0) {
		q--
	}
	return q
}
