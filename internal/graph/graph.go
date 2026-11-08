// Package graph orders tasks that depend on each other.
package graph

import (
	"context"
	"fmt"
	"strings"

	"shopflow/internal/db"
)

// CycleError is returned by TopoSort when the dependencies contain a cycle, so no valid order exists.
// Detect it with errors.As:
//
//	var cycle *graph.CycleError
//	if errors.As(err, &cycle) { ... cycle.Unresolved ... }
type CycleError struct {
	// Unresolved lists EVERY node that could not be ordered -- the cycle itself plus anything that (transitively)
	// depends on it -- sorted alphabetically.
	Unresolved []string
}

func (e *CycleError) Error() string {
	return fmt.Sprintf("dependency cycle: could not order %s", strings.Join(e.Unresolved, ", "))
}

// LoadPipeline reads the seeded nightly pipeline: every task name -> the names of the tasks it depends on
// (an empty slice if none). GIVEN.
func LoadPipeline(ctx context.Context, d db.DBTX) (map[string][]string, error) {
	pipeline := map[string][]string{}

	names, err := d.QueryContext(ctx, `SELECT name FROM pipeline_tasks ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("load pipeline tasks: %w", err)
	}
	for names.Next() {
		var name string
		if err := names.Scan(&name); err != nil {
			names.Close()
			return nil, fmt.Errorf("scan pipeline task: %w", err)
		}
		pipeline[name] = []string{}
	}
	if err := names.Err(); err != nil {
		names.Close()
		return nil, err
	}
	names.Close() // finished with this result set BEFORE starting the next query

	deps, err := d.QueryContext(ctx, `SELECT task, depends_on FROM pipeline_dependencies ORDER BY task, depends_on`)
	if err != nil {
		return nil, fmt.Errorf("load pipeline dependencies: %w", err)
	}
	defer deps.Close()
	for deps.Next() {
		var task, dependsOn string
		if err := deps.Scan(&task, &dependsOn); err != nil {
			return nil, fmt.Errorf("scan pipeline dependency: %w", err)
		}
		pipeline[task] = append(pipeline[task], dependsOn)
	}
	return pipeline, deps.Err()
}
