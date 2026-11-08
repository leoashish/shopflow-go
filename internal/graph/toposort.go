package graph

import "shopflow/internal/todo"

// TopoSort -- STEP 11 (part 1): order tasks with KAHN'S ALGORITHM so that every task comes AFTER everything it depends on.
// (Build systems, database migrations and our nightly pipeline all need exactly this.)
//
// dependsOn maps a node to the nodes it DEPENDS ON ("clean_orders" -> ["extract_orders"]).
//   - Result: every node exactly once, each after all of its dependencies.
//   - Nodes that only appear as someone's dependency (never as a key) are part of the graph too.
//   - Duplicate entries in a dependency list are ignored (counting them twice would fake a cycle).
//   - DETERMINISTIC: whenever several nodes are ready at once, take the alphabetically smallest first. A min-heap of
//     strings (container/heap, as in Step 10) is a neat ready-queue.
//   - A cycle (a self-dependency counts) -> return a *CycleError whose Unresolved lists every node that could NOT be
//     ordered, sorted alphabetically.
//   - Empty graph -> an empty (non-nil) slice.
//
// Kahn's algorithm in words:
//  1. For each node count its unmet dependencies (its "in-degree") and remember, for each node, who is waiting for it.
//  2. Put every node with in-degree 0 into the ready queue.
//  3. Repeatedly take a ready node, append it to the result, and decrease the in-degree of everything waiting for it;
//     when one reaches 0 it becomes ready.
//  4. If the result has fewer nodes than the graph, the rest are stuck behind a cycle.
//
// Cycle detection falls out for free -- no separate search needed. Complexity O(V + E) plus the heap's log factor.
func TopoSort(dependsOn map[string][]string) ([]string, error) {
	return nil, todo.NotImplemented("Step 11: graph.TopoSort")
}
