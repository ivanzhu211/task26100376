package weighplan

import (
	"errors"
	"math/rand"
	"sort"
	"testing"
)

// checkTicket 校验称量单满足总重、步长、允差与完备性约束（精确整数语义）。
func checkTicket(t *testing.T, total int64, ings []Ingredient, tk *Ticket) {
	t.Helper()
	if len(tk.Items) != len(ings) {
		t.Fatalf("ticket has %d items, want %d", len(tk.Items), len(ings))
	}
	byID := make(map[int]Ingredient, len(ings))
	for _, ing := range ings {
		byID[ing.ID] = ing
	}
	var sum, devNum int64
	for i, it := range tk.Items {
		if i > 0 && tk.Items[i-1].ID >= it.ID {
			t.Fatalf("items not sorted by ID: %v", tk.Items)
		}
		ing, ok := byID[it.ID]
		if !ok {
			t.Fatalf("ticket contains unknown ingredient %d", it.ID)
		}
		if it.Amount <= 0 || it.Amount%ing.Step != 0 {
			t.Fatalf("ingredient %d amount %d is not a positive multiple of step %d", it.ID, it.Amount, ing.Step)
		}
		if d := abs64(it.Amount*10000 - total*int64(ing.Target)); d > total*int64(ing.Tolerance) {
			t.Fatalf("ingredient %d amount %d exceeds tolerance (diff %d > %d)", it.ID, it.Amount, d, total*int64(ing.Tolerance))
		}
		sum += it.Amount
		devNum += abs64(it.Amount*10000 - total*int64(ing.Target))
	}
	if sum != total {
		t.Fatalf("ticket total %d != M %d", sum, total)
	}
	if tk.Total() != total {
		t.Fatalf("Ticket.Total() %d != M %d", tk.Total(), total)
	}
	if devNum != tk.DeviationNum {
		t.Fatalf("DeviationNum %d, recomputed %d", tk.DeviationNum, devNum)
	}
}

// TestNearestRoundingFailsButCombinationExists 覆盖场景一：
// 每种原料各自就近取整到步长倍数后总重不能闭合，但存在合法组合。
//
// M=1000，两种原料目标均为 50%，允差 ±5 克（万分之一 50），
// A 步长 6、B 步长 4。理论投料均为 500：
// A 就近取 498、B 就近取 500，合计 998 ≠ 1000；
// 唯一合法组合为 A=504、B=496。
func TestNearestRoundingFailsButCombinationExists(t *testing.T) {
	ings := []Ingredient{
		{ID: 1, Target: 5000, Tolerance: 50, Step: 6},
		{ID: 2, Target: 5000, Tolerance: 50, Step: 4},
	}
	tk, err := Plan(1000, ings)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	checkTicket(t, 1000, ings, tk)
	want := []WeighItem{{ID: 1, Amount: 504}, {ID: 2, Amount: 496}}
	for i, w := range want {
		if tk.Items[i] != w {
			t.Fatalf("item %d = %+v, want %+v", i, tk.Items[i], w)
		}
	}
	if tk.DeviationNum != 80000 { // (|504-500| + |496-500|) * 10000
		t.Fatalf("DeviationNum = %d, want 80000", tk.DeviationNum)
	}
}

// TestTieBrokenLexicographicallyOnToleranceBoundary 覆盖场景二：
// 允差边界上存在多个总偏差相同的最优方案，按编号升序的投料量字典序选择。
//
// M=1000，A 目标 50.20%、B 目标 49.80%，允差均为 ±2 克（万分之一 20），
// 步长均为 4。理论投料 A=502、B=498，允差窗口恰为 [500,504] 与 [496,500]，
// 候选投料量全部落在允差边界上。合法组合 (500,500) 与 (504,496)
// 总偏差相同（均为 4 克），字典序最小者为 A=500、B=500。
func TestTieBrokenLexicographicallyOnToleranceBoundary(t *testing.T) {
	ings := []Ingredient{
		{ID: 1, Target: 5020, Tolerance: 20, Step: 4},
		{ID: 2, Target: 4980, Tolerance: 20, Step: 4},
	}
	tk, err := Plan(1000, ings)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	checkTicket(t, 1000, ings, tk)
	want := []WeighItem{{ID: 1, Amount: 500}, {ID: 2, Amount: 500}}
	for i, w := range want {
		if tk.Items[i] != w {
			t.Fatalf("item %d = %+v, want %+v", i, tk.Items[i], w)
		}
	}
	if tk.DeviationNum != 40000 { // (|500-502| + |500-498|) * 10000
		t.Fatalf("DeviationNum = %d, want 40000", tk.DeviationNum)
	}
}

func TestNoSolution(t *testing.T) {
	// 允差为 0 要求投料量恰为 50，但 50 不是步长 3 的倍数。
	ings := []Ingredient{
		{ID: 1, Target: 5000, Tolerance: 0, Step: 3},
		{ID: 2, Target: 5000, Tolerance: 50, Step: 1},
	}
	if _, err := Plan(100, ings); !errors.Is(err, ErrNoSolution) {
		t.Fatalf("err = %v, want ErrNoSolution", err)
	}

	// 各原料最大可投量之和仍小于 M。
	ings2 := []Ingredient{
		{ID: 1, Target: 5000, Tolerance: 0, Step: 1},
		{ID: 2, Target: 5000, Tolerance: 0, Step: 1},
	}
	if _, err := Plan(101, ings2); !errors.Is(err, ErrNoSolution) {
		t.Fatalf("err = %v, want ErrNoSolution", err)
	}
}

func TestInvalidInput(t *testing.T) {
	base := []Ingredient{
		{ID: 1, Target: 5000, Tolerance: 10, Step: 1},
		{ID: 2, Target: 5000, Tolerance: 10, Step: 1},
	}
	cases := []struct {
		name  string
		total int64
		ings  []Ingredient
	}{
		{"zero total", 0, base},
		{"empty ingredients", 100, nil},
		{"duplicate ID", 100, []Ingredient{base[0], {ID: 1, Target: 5000, Tolerance: 10, Step: 1}}},
		{"non-positive ID", 100, []Ingredient{base[0], {ID: 0, Target: 5000, Tolerance: 10, Step: 1}}},
		{"targets not summing", 100, []Ingredient{base[0], {ID: 2, Target: 5001, Tolerance: 10, Step: 1}}},
		{"zero target", 100, []Ingredient{{ID: 1, Target: 0, Tolerance: 10, Step: 1}, {ID: 2, Target: 10000, Tolerance: 10, Step: 1}}},
		{"tolerance too large", 100, []Ingredient{base[0], {ID: 2, Target: 5000, Tolerance: 51, Step: 1}}},
		{"zero step", 100, []Ingredient{base[0], {ID: 2, Target: 5000, Tolerance: 10, Step: 0}}},
	}
	for _, c := range cases {
		if _, err := Plan(c.total, c.ings); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("%s: err = %v, want ErrInvalidInput", c.name, err)
		}
	}
}

// bruteForce 穷举全部合法称量单，返回最优（总偏差最小、并列字典序最小）。
func bruteForce(total int64, ings []Ingredient) (devNum int64, amounts []int64, ok bool) {
	sorted := make([]Ingredient, len(ings))
	copy(sorted, ings)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].ID < sorted[j].ID })

	ranges := make([][]int64, len(sorted))
	for i, ing := range sorted {
		lo, hi, ok := feasibleRange(total, ing)
		if !ok {
			return 0, nil, false
		}
		for a := lo; a <= hi; a += ing.Step {
			ranges[i] = append(ranges[i], a)
		}
	}
	cur := make([]int64, len(sorted))
	var best []int64
	var bestDev int64
	var rec func(i int, sum int64)
	rec = func(i int, sum int64) {
		if i == len(sorted) {
			if sum != total {
				return
			}
			var d int64
			for j, ing := range sorted {
				d += abs64(cur[j]*10000 - total*int64(ing.Target))
			}
			if best == nil || d < bestDev || (d == bestDev && lessAmounts(cur, best)) {
				bestDev = d
				best = append([]int64{}, cur...)
			}
			return
		}
		for _, a := range ranges[i] {
			if sum+a > total {
				break
			}
			cur[i] = a
			rec(i+1, sum+a)
		}
	}
	rec(0, 0)
	if best == nil {
		return 0, nil, false
	}
	return bestDev, best, true
}

// TestAgainstBruteForce 随机生成小规模配方，与穷举结果对照，
// 验证全局最优与并列选择规则；同时校验称量单全部约束。
func TestAgainstBruteForce(t *testing.T) {
	rng := rand.New(rand.NewSource(20261003))
	for trial := 0; trial < 3000; trial++ {
		n := 1 + rng.Intn(4)
		// 随机切分 10000 为 n 个正整数目标占比。
		cuts := map[int]bool{0: true, 10000: true}
		for len(cuts) < n+1 {
			cuts[1+rng.Intn(9999)] = true
		}
		pts := make([]int, 0, len(cuts))
		for p := range cuts {
			pts = append(pts, p)
		}
		sort.Ints(pts)
		ings := make([]Ingredient, n)
		for i := range ings {
			ings[i] = Ingredient{
				ID:        i + 1,
				Target:    pts[i+1] - pts[i],
				Tolerance: rng.Intn(51),
				Step:      1 + int64(rng.Intn(12)),
			}
		}
		total := int64(1 + rng.Intn(300))

		tk, err := Plan(total, ings)
		wantDev, wantAmounts, wantOK := bruteForce(total, ings)
		if !wantOK {
			if !errors.Is(err, ErrNoSolution) {
				t.Fatalf("trial %d: err = %v, want ErrNoSolution (ings=%+v total=%d)", trial, err, ings, total)
			}
			continue
		}
		if err != nil {
			t.Fatalf("trial %d: Plan: %v (ings=%+v total=%d)", trial, err, ings, total)
		}
		checkTicket(t, total, ings, tk)
		if tk.DeviationNum != wantDev {
			t.Fatalf("trial %d: DeviationNum = %d, want %d (ings=%+v total=%d)", trial, tk.DeviationNum, wantDev, ings, total)
		}
		for i, it := range tk.Items {
			if it.Amount != wantAmounts[i] {
				t.Fatalf("trial %d: item %d amount = %d, want %d (ings=%+v total=%d)", trial, i, it.Amount, wantAmounts[i], ings, total)
			}
		}

		// 打乱输入顺序，结果必须一致。
		shuffled := make([]Ingredient, n)
		perm := rng.Perm(n)
		for i, j := range perm {
			shuffled[i] = ings[j]
		}
		tk2, err := Plan(total, shuffled)
		if err != nil {
			t.Fatalf("trial %d: shuffled Plan: %v", trial, err)
		}
		if tk2.DeviationNum != tk.DeviationNum {
			t.Fatalf("trial %d: shuffled DeviationNum = %d, want %d", trial, tk2.DeviationNum, tk.DeviationNum)
		}
		for i := range tk.Items {
			if tk2.Items[i] != tk.Items[i] {
				t.Fatalf("trial %d: shuffled item %d = %+v, want %+v", trial, i, tk2.Items[i], tk.Items[i])
			}
		}
	}
}
