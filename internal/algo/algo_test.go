package algo_test

import (
	"math/rand"
	"slices"
	"sort"
	"sync"
	"testing"
	"time"

	"shopflow/internal/algo"
	"shopflow/internal/testutil"
)

// Step 10 (parts 1 and 2) -- heaps and caches.   Run:  go test ./internal/algo ./internal/analytics -run Step10 -race -v

// ======================= TopK =======================

func topK[T interface{ ~int | ~int64 | ~string }](t *testing.T, k int, entries ...algo.Scored[T]) []algo.Scored[T] {
	t.Helper()
	tk, err := algo.NewTopK[T](k)
	if err != nil {
		t.Fatalf("NewTopK(%d): %v", k, err)
	}
	for _, e := range entries {
		tk.Add(e.Item, e.Score)
	}
	return tk.Results()
}

func TestStep10_TopK_ReturnsTheKHighestScores_BestFirst(t *testing.T) {
	got := topK(t, 3,
		algo.Scored[string]{Item: "a", Score: 5}, algo.Scored[string]{Item: "b", Score: 9}, algo.Scored[string]{Item: "c", Score: 1},
		algo.Scored[string]{Item: "d", Score: 7}, algo.Scored[string]{Item: "e", Score: 3})

	want := []algo.Scored[string]{{Item: "b", Score: 9}, {Item: "d", Score: 7}, {Item: "a", Score: 5}}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestStep10_TopK_BreaksTiesBySmallerItemFirst(t *testing.T) {
	got := topK(t, 2,
		algo.Scored[string]{Item: "c", Score: 5}, algo.Scored[string]{Item: "a", Score: 5},
		algo.Scored[string]{Item: "b", Score: 5}, algo.Scored[string]{Item: "d", Score: 5})

	want := []algo.Scored[string]{{Item: "a", Score: 5}, {Item: "b", Score: 5}}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestStep10_TopK_ReturnsEverythingOrdered_WhenKIsLargerThanTheInput(t *testing.T) {
	got := topK(t, 10, algo.Scored[string]{Item: "x", Score: 1}, algo.Scored[string]{Item: "y", Score: 3}, algo.Scored[string]{Item: "z", Score: 2})

	want := []algo.Scored[string]{{Item: "y", Score: 3}, {Item: "z", Score: 2}, {Item: "x", Score: 1}}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestStep10_TopK_ReturnsAnEmptyNonNilSlice_ForKZeroOrNoInput(t *testing.T) {
	if got := topK(t, 0, algo.Scored[string]{Item: "a", Score: 1}); got == nil || len(got) != 0 {
		t.Fatalf("k=0: got %v, want an empty non-nil slice", got)
	}
	if got := topK[string](t, 5); got == nil || len(got) != 0 {
		t.Fatalf("no input: got %v, want an empty non-nil slice", got)
	}
}

func TestStep10_TopK_RejectsANegativeK(t *testing.T) {
	_, err := algo.NewTopK[string](-1)
	testutil.ExpectError(t, err)
}

func TestStep10_TopK_ResultsCanBeCalledRepeatedly_AndAddCanContinue(t *testing.T) {
	tk, _ := algo.NewTopK[int](2)
	tk.Add(1, 10)
	tk.Add(2, 20)

	first := tk.Results()
	second := tk.Results()
	if !slices.Equal(first, second) || len(first) != 2 {
		t.Fatalf("Results is not repeatable: %v then %v", first, second)
	}

	tk.Add(3, 30) // still works after Results
	want := []algo.Scored[int]{{Item: 3, Score: 30}, {Item: 2, Score: 20}}
	if got := tk.Results(); !slices.Equal(got, want) {
		t.Fatalf("after another Add: got %v, want %v", got, want)
	}
}

func TestStep10_TopK_AgreesWithAFullSort_OnLargeRandomInput(t *testing.T) {
	rnd := rand.New(rand.NewSource(1234))
	all := make([]algo.Scored[int], 5000)
	tk, _ := algo.NewTopK[int](10)
	for i := range all {
		all[i] = algo.Scored[int]{Item: i, Score: int64(rnd.Intn(200))} // plenty of ties
		tk.Add(all[i].Item, all[i].Score)
	}

	sort.Slice(all, func(i, j int) bool {
		if all[i].Score != all[j].Score {
			return all[i].Score > all[j].Score
		}
		return all[i].Item < all[j].Item
	})

	if got := tk.Results(); !slices.Equal(got, all[:10]) {
		t.Fatalf("heap result differs from the full sort:\n got  %v\n want %v", got, all[:10])
	}
}

// ======================= LRU =======================

func newLRU(t *testing.T, capacity int) *algo.LRU[string, int] {
	t.Helper()
	c, err := algo.NewLRU[string, int](capacity)
	if err != nil {
		t.Fatalf("NewLRU(%d): %v", capacity, err)
	}
	return c
}

func TestStep10_LRU_StoresAndReturnsValues(t *testing.T) {
	c := newLRU(t, 2)
	c.Put("a", 1)

	if v, ok := c.Get("a"); !ok || v != 1 {
		t.Fatalf("Get(a) = (%d, %v), want (1, true)", v, ok)
	}
	if v, ok := c.Get("missing"); ok || v != 0 {
		t.Fatalf("Get(missing) = (%d, %v), want (0, false)", v, ok)
	}
	if c.Len() != 1 {
		t.Fatalf("Len = %d, want 1", c.Len())
	}
}

func TestStep10_LRU_EvictsTheLeastRecentlyUsedEntry_WhenFull(t *testing.T) {
	c := newLRU(t, 2)
	c.Put("a", 1)
	c.Put("b", 2)
	c.Put("c", 3) // evicts "a"

	if c.Contains("a") || !c.Contains("b") || !c.Contains("c") || c.Len() != 2 {
		t.Fatalf("contains a=%v b=%v c=%v len=%d, want a evicted, b and c kept, len 2", c.Contains("a"), c.Contains("b"), c.Contains("c"), c.Len())
	}
}

func TestStep10_LRU_ReadingAnEntry_MakesItRecent(t *testing.T) {
	c := newLRU(t, 2)
	c.Put("a", 1)
	c.Put("b", 2)
	c.Get("a") // "b" is now the least recently used
	c.Put("c", 3)

	if !c.Contains("a") || c.Contains("b") {
		t.Fatalf("contains a=%v b=%v, want a kept and b evicted", c.Contains("a"), c.Contains("b"))
	}
}

func TestStep10_LRU_PuttingAnExistingKey_UpdatesTheValue_RefreshesIt_AndDoesNotGrow(t *testing.T) {
	c := newLRU(t, 2)
	c.Put("a", 1)
	c.Put("b", 2)
	c.Put("a", 100) // update: "b" becomes the oldest
	c.Put("c", 3)

	if c.Len() != 2 || c.Contains("b") {
		t.Fatalf("len=%d contains b=%v, want len 2 and b evicted", c.Len(), c.Contains("b"))
	}
	if v, ok := c.Get("a"); !ok || v != 100 {
		t.Fatalf("Get(a) = (%d, %v), want (100, true)", v, ok)
	}
}

func TestStep10_LRU_Contains_DoesNotChangeRecency(t *testing.T) {
	c := newLRU(t, 2)
	c.Put("a", 1)
	c.Put("b", 2)
	if !c.Contains("a") { // a peek: "a" stays the oldest
		t.Fatal("Contains(a) = false")
	}
	c.Put("c", 3)

	if c.Contains("a") || !c.Contains("b") {
		t.Fatalf("contains a=%v b=%v, want a evicted (Contains must not refresh it) and b kept", c.Contains("a"), c.Contains("b"))
	}
}

func TestStep10_LRU_WorksWithCapacityOne(t *testing.T) {
	c, err := algo.NewLRU[int, string](1)
	if err != nil {
		t.Fatal(err)
	}
	c.Put(1, "one")
	c.Put(2, "two")

	if c.Contains(1) {
		t.Fatal("key 1 should have been evicted")
	}
	if v, ok := c.Get(2); !ok || v != "two" {
		t.Fatalf("Get(2) = (%q, %v), want (two, true)", v, ok)
	}
}

func TestStep10_LRU_RejectsACapacityBelowOne(t *testing.T) {
	for _, capacity := range []int{0, -1} {
		_, err := algo.NewLRU[string, int](capacity)
		testutil.ExpectError(t, err)
	}
}

func TestStep10_LRU_IsSafeForConcurrentUse(t *testing.T) {
	c, err := algo.NewLRU[int, int](50)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for w := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rnd := rand.New(rand.NewSource(int64(w)))
			for range 5000 {
				key := rnd.Intn(200)
				if rnd.Intn(2) == 0 {
					c.Put(key, key*2)
				} else if v, ok := c.Get(key); ok && v != key*2 {
					t.Errorf("Get(%d) = %d, want %d", key, v, key*2)
					return
				}
			}
		}()
	}
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	testutil.WaitClosed(t, done, 30*time.Second)

	if n := c.Len(); n < 1 || n > 50 {
		t.Fatalf("Len = %d, want between 1 and 50", n)
	}
}
