package graph_test

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"slices"
	"testing"

	"shopflow/internal/graph"
	"shopflow/internal/testutil"
)

// Step 11 (part 1) -- topological sort.   Run:  go test ./internal/graph ./internal/ratelimit -run Step11 -race -v

func sortOK(t *testing.T, deps map[string][]string) []string {
	t.Helper()
	got, err := graph.TopoSort(deps)
	if err != nil {
		t.Fatalf("TopoSort: %v", err)
	}
	return got
}

func TestStep11_TopoSort_PutsEveryNodeAfterItsDependencies(t *testing.T) {
	got := sortOK(t, map[string][]string{"c": {"b"}, "b": {"a"}, "a": {}})

	if want := []string{"a", "b", "c"}; !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestStep11_TopoSort_BreaksTiesAlphabetically_SoTheResultIsDeterministic(t *testing.T) {
	diamond := sortOK(t, map[string][]string{"d": {"b", "c"}, "b": {"a"}, "c": {"a"}, "a": {}})
	if want := []string{"a", "b", "c", "d"}; !slices.Equal(diamond, want) {
		t.Fatalf("diamond: got %v, want %v", diamond, want)
	}

	independent := sortOK(t, map[string][]string{"z": {}, "m": {}, "a": {}})
	if want := []string{"a", "m", "z"}; !slices.Equal(independent, want) {
		t.Fatalf("independent: got %v, want %v", independent, want)
	}
}

func TestStep11_TopoSort_IncludesNodesThatOnlyAppearAsDependencies(t *testing.T) {
	if got, want := sortOK(t, map[string][]string{"b": {"a"}}), []string{"a", "b"}; !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestStep11_TopoSort_IgnoresDuplicateDependencyEntries(t *testing.T) {
	if got, want := sortOK(t, map[string][]string{"b": {"a", "a"}}), []string{"a", "b"}; !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestStep11_TopoSort_ReturnsAnEmptyNonNilSlice_ForAnEmptyGraph(t *testing.T) {
	if got := sortOK(t, map[string][]string{}); got == nil || len(got) != 0 {
		t.Fatalf("got %v, want an empty non-nil slice", got)
	}
}

func unresolved(t *testing.T, deps map[string][]string) []string {
	t.Helper()
	_, err := graph.TopoSort(deps)
	var cycle *graph.CycleError
	if !errors.As(err, &cycle) {
		t.Fatalf("error = %v, want a *graph.CycleError", err)
	}
	return cycle.Unresolved
}

func TestStep11_TopoSort_ReportsACycle(t *testing.T) {
	got := unresolved(t, map[string][]string{"a": {"b"}, "b": {"c"}, "c": {"a"}, "d": {}})

	if want := []string{"a", "b", "c"}; !slices.Equal(got, want) { // "d" is fine; the cycle is not
		t.Fatalf("Unresolved = %v, want %v", got, want)
	}
}

func TestStep11_TopoSort_TreatsASelfDependencyAsACycle(t *testing.T) {
	if got, want := unresolved(t, map[string][]string{"a": {"a"}}), []string{"a"}; !slices.Equal(got, want) {
		t.Fatalf("Unresolved = %v, want %v", got, want)
	}
}

func TestStep11_TopoSort_ReportsNodesStuckBehindACycle_Too(t *testing.T) {
	// e depends on a, which is part of the a<->b cycle, so e can never run either.
	got := unresolved(t, map[string][]string{"a": {"b"}, "b": {"a"}, "e": {"a"}, "ok": {}})

	if want := []string{"a", "b", "e"}; !slices.Equal(got, want) {
		t.Fatalf("Unresolved = %v, want %v", got, want)
	}
}

func TestStep11_TopoSort_OrdersALargeRandomDAG_Correctly(t *testing.T) {
	rnd := rand.New(rand.NewSource(99))
	deps := map[string][]string{}
	for i := range 300 {
		var d []string
		for j := range i { // node i may only depend on LOWER-numbered nodes, so the graph is guaranteed acyclic
			if rnd.Intn(100) < 3 {
				d = append(d, fmt.Sprintf("n%03d", j))
			}
		}
		deps[fmt.Sprintf("n%03d", i)] = d
	}

	order := sortOK(t, deps)

	if len(order) != 300 {
		t.Fatalf("got %d nodes, want 300", len(order))
	}
	position := map[string]int{}
	for i, n := range order {
		if _, dup := position[n]; dup {
			t.Fatalf("node %s appears twice", n)
		}
		position[n] = i
	}
	for node, d := range deps {
		for _, dep := range d {
			if position[dep] >= position[node] {
				t.Fatalf("%s must come before %s", dep, node)
			}
		}
	}
}

func TestStep11_TopoSort_OrdersTheSeededNightlyPipeline(t *testing.T) {
	pipeline, err := graph.LoadPipeline(context.Background(), testutil.NewDB(t))
	if err != nil {
		t.Fatalf("LoadPipeline: %v", err)
	}

	got := sortOK(t, pipeline)

	want := []string{
		"extract_customers", "extract_orders", "clean_orders", "extract_products", "compute_best_sellers",
		"join_orders_customers", "compute_revenue", "publish_dashboard", "send_report_email", "archive_raw",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("got  %v\nwant %v", got, want)
	}
}
