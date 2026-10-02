package weighplan

import (
	"errors"
	"math/rand"
	"sort"
	"testing"
)

// checkFeasible 校验称量单满足总重、步长、允差与全原料覆盖约束。
func checkFeasible(t *testing.T, M int64, ings []Ingredient, p Plan) {
	t.Helper()
	if len(p.Items) != len(ings) {
		t.Fatalf("plan has %d items, want %d", len(p.Items), len(ings))
	}
	byID := make(map[int64]Ingredient, len(ings))
	for _, ing := range ings {
		byID[ing.ID] = ing
	}
	var sum int64
	for i, it := range p.Items {
		if i > 0 && p.Items[i-1].ID >= it.ID {
			t.Fatalf("items not sorted by ascending ID: %v", p.Items)
		}
		ing, ok := byID[it.ID]
		if !ok {
			t.Fatalf("plan contains unknown ingredient %d", it.ID)
		}
		if it.Amount <= 0 || it.Amount%ing.StepGrams != 0 {
			t.Fatalf("ingredient %d amount %d is not a positive multiple of step %d", it.ID, it.Amount, ing.StepGrams)
		}
		if d := abs(it.Amount*10000 - M*ing.TargetPpm); d > M*ing.TolerancePpm {
			t.Fatalf("ingredient %d amount %d violates tolerance (dev %d > %d)", it.ID, it.Amount, d, M*ing.TolerancePpm)
		}
		sum += it.Amount
	}
	if sum != M {
		t.Fatalf("total weight %d != M %d", sum, M)
	}
	var dev int64
	for _, it := range p.Items {
		dev += abs(it.Amount*10000 - M*byID[it.ID].TargetPpm)
	}
	if dev != p.TotalDeviation {
		t.Fatalf("reported deviation %d != recomputed %d", p.TotalDeviation, dev)
	}
}

func abs(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}

// bruteForce 用穷举求精确最优解（最小总偏差，并列取投料量字典序最小），供交叉验证。
func bruteForce(M int64, ings []Ingredient) (amounts []int64, dev int64, ok bool) {
	sorted := make([]Ingredient, len(ings))
	copy(sorted, ings)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].ID < sorted[j].ID })

	n := len(sorted)
	choices := make([][]int64, n)
	for i, ing := range sorted {
		lo, hi := bounds(M, ing)
		if lo < ing.StepGrams {
			lo = ing.StepGrams
		}
		for a := ceilDiv(lo, ing.StepGrams) * ing.StepGrams; a <= hi; a += ing.StepGrams {
			choices[i] = append(choices[i], a)
		}
	}
	best := append([]int64(nil), amounts...)
	bestDev := int64(-1)
	cur := make([]int64, n)
	var rec func(i int, sum int64)
	rec = func(i int, sum int64) {
		if i == n {
			if sum != M {
				return
			}
			var d int64
			for j, a := range cur {
				d += abs(a*10000 - M*sorted[j].TargetPpm)
			}
			if bestDev < 0 || d < bestDev || (d == bestDev && less(cur, best)) {
				bestDev = d
				best = append([]int64(nil), cur...)
			}
			return
		}
		for _, a := range choices[i] {
			if sum+a > M {
				break
			}
			cur[i] = a
			rec(i+1, sum+a)
		}
	}
	rec(0, 0)
	if bestDev < 0 {
		return nil, 0, false
	}
	return best, bestDev, true
}

func less(a, b []int64) bool {
	for i := range a {
		if a[i] != b[i] {
			return a[i] < b[i]
		}
	}
	return false
}

// 逐项就近取整无法闭合（500+498=998、500+504=1004），但存在合法组合 496+504=1000。
func TestNearestRoundingFailsButFeasible(t *testing.T) {
	ings := []Ingredient{
		{ID: 1, TargetPpm: 5000, TolerancePpm: 50, StepGrams: 4},
		{ID: 2, TargetPpm: 5000, TolerancePpm: 50, StepGrams: 6},
	}
	p, err := PlanBatch(1000, ings)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	checkFeasible(t, 1000, ings, p)
	want := []Item{{ID: 1, Amount: 496}, {ID: 2, Amount: 504}}
	if !equalItems(p.Items, want) {
		t.Fatalf("got %v, want %v", p.Items, want)
	}
	if p.TotalDeviation != 80000 { // (4+4)g，单位 1/10000 g
		t.Fatalf("deviation %d, want 80000", p.TotalDeviation)
	}
}

// 允差边界上存在两个最优方案 (495,500,5) 与 (505,490,5)，总偏差均为 10g，
// 且均为边界取值；应选择字典序最小的 (495,500,5)。
func TestBoundaryTiePicksLexicographicallySmallest(t *testing.T) {
	ings := []Ingredient{
		{ID: 1, TargetPpm: 5000, TolerancePpm: 50, StepGrams: 5},
		{ID: 2, TargetPpm: 4950, TolerancePpm: 50, StepGrams: 10},
		{ID: 3, TargetPpm: 50, TolerancePpm: 0, StepGrams: 5},
	}
	p, err := PlanBatch(1000, ings)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	checkFeasible(t, 1000, ings, p)
	want := []Item{{ID: 1, Amount: 495}, {ID: 2, Amount: 500}, {ID: 3, Amount: 5}}
	if !equalItems(p.Items, want) {
		t.Fatalf("got %v, want %v", p.Items, want)
	}
	if p.TotalDeviation != 100000 { // (5+5+0)g，单位 1/10000 g
		t.Fatalf("deviation %d, want 100000", p.TotalDeviation)
	}
	// 确认并列的另一方案 (505,490,5) 确实同样最优，以验证测试场景有效。
	alt := []int64{505, 490, 5}
	var altDev int64
	for i, a := range alt {
		altDev += abs(a*10000 - 1000*ings[i].TargetPpm)
	}
	if altDev != p.TotalDeviation {
		t.Fatalf("alternative plan deviation %d, want tie at %d", altDev, p.TotalDeviation)
	}
}

func TestNoSolution(t *testing.T) {
	ings := []Ingredient{
		{ID: 1, TargetPpm: 5000, TolerancePpm: 50, StepGrams: 4},
		{ID: 2, TargetPpm: 5000, TolerancePpm: 50, StepGrams: 6},
	}
	// 所有合法投料量均为偶数，总重必为偶数，M=1001 无解。
	if _, err := PlanBatch(1001, ings); !errors.Is(err, ErrNoSolution) {
		t.Fatalf("got %v, want ErrNoSolution", err)
	}
	// 步长超过允差区间，单个原料即无可行投料量。
	ings2 := []Ingredient{
		{ID: 1, TargetPpm: 5000, TolerancePpm: 0, StepGrams: 7},
		{ID: 2, TargetPpm: 5000, TolerancePpm: 50, StepGrams: 1},
	}
	if _, err := PlanBatch(1000, ings2); !errors.Is(err, ErrNoSolution) {
		t.Fatalf("got %v, want ErrNoSolution", err)
	}
}

func TestInputOrderDoesNotMatter(t *testing.T) {
	ings := []Ingredient{
		{ID: 7, TargetPpm: 2000, TolerancePpm: 30, StepGrams: 3},
		{ID: 3, TargetPpm: 5000, TolerancePpm: 50, StepGrams: 4},
		{ID: 9, TargetPpm: 3000, TolerancePpm: 10, StepGrams: 7},
	}
	base, err := PlanBatch(5000, ings)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	checkFeasible(t, 5000, ings, base)
	perm := []Ingredient{ings[1], ings[2], ings[0]}
	for i := 0; i < 6; i++ {
		got, err := PlanBatch(5000, perm)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !equalItems(got.Items, base.Items) || got.TotalDeviation != base.TotalDeviation {
			t.Fatalf("permutation %v changed result: %v vs %v", perm, got, base)
		}
		perm[0], perm[1] = perm[1], perm[0]
		perm[1], perm[2] = perm[2], perm[1]
	}
}

// 随机小规模用例与穷举结果逐一对比：有解时必须一致，无解时必须报 ErrNoSolution。
func TestBruteForceCrossCheck(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	for trial := 0; trial < 300; trial++ {
		n := 2 + rng.Intn(3)
		// 随机切分总和为 10000 的正整数目标占比。
		cuts := map[int]bool{0: true, 10000: true}
		for len(cuts) < n+1 {
			cuts[1+rng.Intn(9999)] = true
		}
		points := make([]int, 0, len(cuts))
		for c := range cuts {
			points = append(points, c)
		}
		sort.Ints(points)
		ings := make([]Ingredient, n)
		for i := range ings {
			ings[i] = Ingredient{
				ID:           int64(i + 1),
				TargetPpm:    int64(points[i+1] - points[i]),
				TolerancePpm: int64(rng.Intn(51)),
				StepGrams:    int64(1 + rng.Intn(9)),
			}
		}
		M := int64(50 + rng.Intn(950))
		p, err := PlanBatch(M, ings)
		wantAmounts, wantDev, ok := bruteForce(M, ings)
		if !ok {
			if !errors.Is(err, ErrNoSolution) {
				t.Fatalf("trial %d: got %v, want ErrNoSolution (M=%d, ings=%v)", trial, err, M, ings)
			}
			continue
		}
		if err != nil {
			t.Fatalf("trial %d: unexpected error %v (M=%d, ings=%v)", trial, err, M, ings)
		}
		checkFeasible(t, M, ings, p)
		if p.TotalDeviation != wantDev {
			t.Fatalf("trial %d: deviation %d, want %d (M=%d, ings=%v)", trial, p.TotalDeviation, wantDev, M, ings)
		}
		for i, it := range p.Items {
			if it.Amount != wantAmounts[i] {
				t.Fatalf("trial %d: amounts %v, want %v (M=%d, ings=%v)", trial, p.Items, wantAmounts, M, ings)
			}
		}
	}
}

func TestInvalidInput(t *testing.T) {
	good := []Ingredient{
		{ID: 1, TargetPpm: 5000, TolerancePpm: 50, StepGrams: 4},
		{ID: 2, TargetPpm: 5000, TolerancePpm: 50, StepGrams: 6},
	}
	cases := []struct {
		name string
		M    int64
		ings []Ingredient
	}{
		{"zero M", 0, good},
		{"negative M", -5, good},
		{"no ingredients", 1000, nil},
		{"duplicate ID", 1000, []Ingredient{good[0], {ID: 1, TargetPpm: 5000, TolerancePpm: 50, StepGrams: 6}}},
		{"non-positive ID", 1000, []Ingredient{good[0], {ID: 0, TargetPpm: 5000, TolerancePpm: 50, StepGrams: 6}}},
		{"zero target", 1000, []Ingredient{good[0], {ID: 2, TargetPpm: 0, TolerancePpm: 50, StepGrams: 6}}},
		{"targets not summing to 10000", 1000, []Ingredient{good[0], {ID: 2, TargetPpm: 5001, TolerancePpm: 50, StepGrams: 6}}},
		{"negative tolerance", 1000, []Ingredient{good[0], {ID: 2, TargetPpm: 5000, TolerancePpm: -1, StepGrams: 6}}},
		{"tolerance above 50", 1000, []Ingredient{good[0], {ID: 2, TargetPpm: 5000, TolerancePpm: 51, StepGrams: 6}}},
		{"zero step", 1000, []Ingredient{good[0], {ID: 2, TargetPpm: 5000, TolerancePpm: 50, StepGrams: 0}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := PlanBatch(tc.M, tc.ings); err == nil || errors.Is(err, ErrNoSolution) {
				t.Fatalf("got %v, want validation error", err)
			}
		})
	}
	// 合法输入不得报错。
	if _, err := PlanBatch(1000, good); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func equalItems(a, b []Item) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
