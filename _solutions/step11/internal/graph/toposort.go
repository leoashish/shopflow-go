package graph

import (
	"container/heap"
	"slices"
	"sort"
)

// stringHeap is a min-heap of strings: the ready-queue that always yields the alphabetically smallest node.
type stringHeap struct{ sort.StringSlice }

func (h *stringHeap) Push(x any) { h.StringSlice = append(h.StringSlice, x.(string)) }
func (h *stringHeap) Pop() any {
	old := h.StringSlice
	n := len(old)
	x := old[n-1]
	h.StringSlice = old[:n-1]
	return x
}

func TopoSort(dependsOn map[string][]string) ([]string, error) {
	unmet := map[string]int{}           // node -> number of dependencies not yet placed
	dependents := map[string][]string{} // node -> nodes waiting for it

	register := func(node string) {
		if _, known := unmet[node]; !known {
			unmet[node] = 0
		}
	}

	for node, deps := range dependsOn {
		register(node)
		seen := map[string]bool{}
		for _, dep := range deps {
			if seen[dep] {
				continue // duplicate entry: count each dependency once
			}
			seen[dep] = true
			register(dep)
			unmet[node]++
			dependents[dep] = append(dependents[dep], node)
		}
	}

	ready := &stringHeap{}
	for node, count := range unmet {
		if count == 0 {
			heap.Push(ready, node)
		}
	}

	order := make([]string, 0, len(unmet))
	for ready.Len() > 0 {
		node := heap.Pop(ready).(string)
		order = append(order, node)
		for _, waiting := range dependents[node] {
			unmet[waiting]--
			if unmet[waiting] == 0 {
				heap.Push(ready, waiting)
			}
		}
	}

	if len(order) < len(unmet) {
		placed := make(map[string]bool, len(order))
		for _, n := range order {
			placed[n] = true
		}
		var stuck []string
		for node := range unmet {
			if !placed[node] {
				stuck = append(stuck, node)
			}
		}
		slices.Sort(stuck)
		return nil, &CycleError{Unresolved: stuck}
	}
	return order, nil
}
