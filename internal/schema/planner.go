package schema

import (
	"fmt"
	"sort"
)

// QueryPlan represents an execution plan for a query.
// It contains a sequence of steps that may have dependencies on each other.
type QueryPlan struct {
	Steps []*PlanStep
}

// PlanStep represents a single step in the query execution plan.
type PlanStep struct {
	ID           string
	UpstreamName string
	FieldPath    string
	Dependencies []string // IDs of steps that must complete before this one
	Parallel     bool     // Whether this step can run in parallel with others at same depth
	DataExtraction map[string]string // Maps result fields to variables for later steps

	// Execution details
	OperationType string // "query" or "mutation"
	Arguments     map[string]interface{}
}

// PlanBuilder builds query plans from GraphQL operations.
type PlanBuilder struct {
	// Upstream dependencies
	upstreamDeps map[string][]string // field -> upstreams it depends on
}

// NewPlanBuilder creates a new query plan builder.
func NewPlanBuilder() *PlanBuilder {
	return &PlanBuilder{
		upstreamDeps: make(map[string][]string),
	}
}

// RegisterUpstreamDependency registers that a field depends on data from an upstream.
func (pb *PlanBuilder) RegisterUpstreamDependency(fieldPath string, upstreamName string) {
	if pb.upstreamDeps[fieldPath] == nil {
		pb.upstreamDeps[fieldPath] = []string{}
	}
	// Check if already registered
	for _, existing := range pb.upstreamDeps[fieldPath] {
		if existing == upstreamName {
			return
		}
	}
	pb.upstreamDeps[fieldPath] = append(pb.upstreamDeps[fieldPath], upstreamName)
}

// BuildPlan builds a query plan from a GraphQL operation.
// It analyzes field dependencies and creates a topological ordering of steps.
func (pb *PlanBuilder) BuildPlan(operationName string, fields []PlanField) (*QueryPlan, error) {
	plan := &QueryPlan{
		Steps: make([]*PlanStep, 0),
	}

	if len(fields) == 0 {
		return plan, nil
	}

	// Step 1: Group fields by upstream
	fieldsByUpstream := pb.groupFieldsByUpstream(fields)

	// Step 2: Build dependency graph
	depGraph := pb.buildDependencyGraph(fields)

	// Step 3: Topological sort to determine execution order
	orderedUpstreams, err := pb.topologicalSort(fieldsByUpstream, depGraph)
	if err != nil {
		return nil, fmt.Errorf("dependency resolution failed: %w", err)
	}

	// Step 4: Create plan steps
	stepID := 0
	for _, upstreamGroup := range orderedUpstreams {
		// Fields at the same level can run in parallel
		isParallel := len(upstreamGroup) > 1

		for _, upstreamName := range upstreamGroup {
			stepFields := fieldsByUpstream[upstreamName]
			if len(stepFields) == 0 {
				continue
			}

			step := &PlanStep{
				ID:           fmt.Sprintf("step-%d", stepID),
				UpstreamName: upstreamName,
				Parallel:     isParallel,
				Dependencies: pb.getStepDependencies(plan.Steps, upstreamName, depGraph),
				DataExtraction: make(map[string]string),
				OperationType:  pb.inferOperationType(stepFields),
				Arguments:      make(map[string]interface{}),
			}

			// Build field path
			if len(stepFields) == 1 {
				step.FieldPath = stepFields[0].Path
			} else {
				step.FieldPath = fmt.Sprintf("{%s}", joinFieldPaths(stepFields))
			}

			// Extract data bindings for dependent fields
			for _, field := range stepFields {
				for varName, binding := range field.DataBindings {
					step.DataExtraction[varName] = binding
				}
			}

			plan.Steps = append(plan.Steps, step)
			stepID++
		}
	}

	return plan, nil
}

// PlanField represents a field to be included in the query plan.
type PlanField struct {
	Path         string
	UpstreamName string
	Dependencies []string // Other field paths this field depends on
	DataBindings map[string]string // Variable name -> result path binding
	Arguments    map[string]interface{}
}

// groupFieldsByUpstream groups fields by their upstream source.
func (pb *PlanBuilder) groupFieldsByUpstream(fields []PlanField) map[string][]PlanField {
	grouped := make(map[string][]PlanField)
	for _, field := range fields {
		upstream := field.UpstreamName
		if upstream == "" {
			upstream = "default"
		}
		grouped[upstream] = append(grouped[upstream], field)
	}
	return grouped
}

// buildDependencyGraph builds a graph of dependencies between upstreams.
func (pb *PlanBuilder) buildDependencyGraph(fields []PlanField) map[string][]string {
	graph := make(map[string][]string)

	// Initialize all upstreams
	for _, field := range fields {
		upstream := field.UpstreamName
		if upstream == "" {
			upstream = "default"
		}
		if graph[upstream] == nil {
			graph[upstream] = []string{}
		}
	}

	// Add dependencies
	for _, field := range fields {
		upstream := field.UpstreamName
		if upstream == "" {
			upstream = "default"
		}

		for _, dep := range field.Dependencies {
			// Find which upstream provides this dependency
			depUpstream := pb.findUpstreamForField(dep, fields)
			if depUpstream != "" && depUpstream != upstream {
				// Check if already in list
				found := false
				for _, existing := range graph[upstream] {
					if existing == depUpstream {
						found = true
						break
					}
				}
				if !found {
					graph[upstream] = append(graph[upstream], depUpstream)
				}
			}
		}
	}

	return graph
}

// findUpstreamForField finds which upstream provides a given field.
func (pb *PlanBuilder) findUpstreamForField(fieldPath string, fields []PlanField) string {
	for _, field := range fields {
		if field.Path == fieldPath {
			if field.UpstreamName == "" {
				return "default"
			}
			return field.UpstreamName
		}
	}
	return ""
}

// topologicalSort performs a topological sort on upstreams based on dependencies.
// Returns groups of upstreams that can run at the same depth.
func (pb *PlanBuilder) topologicalSort(
	fieldsByUpstream map[string][]PlanField,
	depGraph map[string][]string,
) ([][]string, error) {
	var result [][]string

	// Calculate in-degree for each upstream
	inDegree := make(map[string]int)
	for upstream := range fieldsByUpstream {
		inDegree[upstream] = 0
	}
	for upstream, deps := range depGraph {
		for _, dep := range deps {
			if _, exists := fieldsByUpstream[dep]; exists {
				inDegree[upstream]++
			}
		}
	}

	// Find all sources (in-degree 0)
	var currentLayer []string
	for upstream, degree := range inDegree {
		if degree == 0 {
			currentLayer = append(currentLayer, upstream)
		}
	}

	// Sort for deterministic output
	sort.Strings(currentLayer)

	visited := make(map[string]bool)

	for len(currentLayer) > 0 {
		result = append(result, currentLayer)

		// Mark current layer as visited
		for _, upstream := range currentLayer {
			visited[upstream] = true
		}

		// Find next layer
		nextLayer := []string{}
		for upstream := range fieldsByUpstream {
			if visited[upstream] {
				continue
			}

			// Check if all dependencies are satisfied
			allDepsSatisfied := true
			for _, dep := range depGraph[upstream] {
				if !visited[dep] {
					allDepsSatisfied = false
					break
				}
			}

			if allDepsSatisfied {
				nextLayer = append(nextLayer, upstream)
			}
		}

		sort.Strings(nextLayer)
		currentLayer = nextLayer
	}

	// Check for cycles
	if len(visited) != len(fieldsByUpstream) {
		return nil, fmt.Errorf("cycle detected in query dependencies")
	}

	return result, nil
}

// getStepDependencies finds dependencies for a new step.
func (pb *PlanBuilder) getStepDependencies(existingSteps []*PlanStep, upstreamName string, depGraph map[string][]string) []string {
	var deps []string

	// Find which existing steps provide data we depend on
	for _, depUpstream := range depGraph[upstreamName] {
		for _, step := range existingSteps {
			if step.UpstreamName == depUpstream {
				// Check if already in list
				found := false
				for _, existing := range deps {
					if existing == step.ID {
						found = true
						break
					}
				}
				if !found {
					deps = append(deps, step.ID)
				}
			}
		}
	}

	return deps
}

// inferOperationType determines if this is a query or mutation.
func (pb *PlanBuilder) inferOperationType(fields []PlanField) string {
	// Check if any field indicates a mutation
	for _, field := range fields {
		if field.Path == "Mutation" || hasMutationPrefix(field.Path) {
			return "mutation"
		}
	}
	return "query"
}

// hasMutationPrefix checks if field path indicates a mutation.
func hasMutationPrefix(path string) bool {
	mutationPrefixes := []string{"create", "update", "delete", "remove", "add", "set", "modify"}
	for _, prefix := range mutationPrefixes {
		if len(path) >= len(prefix) && path[:len(prefix)] == prefix {
			return true
		}
	}
	return false
}

// joinFieldPaths joins multiple field paths into a single string.
func joinFieldPaths(fields []PlanField) string {
	if len(fields) == 0 {
		return ""
	}

	result := fields[0].Path
	for i := 1; i < len(fields); i++ {
		result += ", " + fields[i].Path
	}
	return result
}

// NewQueryPlan creates a new empty query plan.
func NewQueryPlan() *QueryPlan {
	return &QueryPlan{
		Steps: make([]*PlanStep, 0),
	}
}

// StepCount returns the number of steps in the plan.
func (p *QueryPlan) StepCount() int {
	return len(p.Steps)
}

// GetStep retrieves a step by ID.
func (p *QueryPlan) GetStep(id string) *PlanStep {
	for _, step := range p.Steps {
		if step.ID == id {
			return step
		}
	}
	return nil
}

// GetStepsAtDepth returns all steps that can run at a given depth.
// Depth 0 = steps with no dependencies, Depth 1 = steps depending on depth 0, etc.
func (p *QueryPlan) GetStepsAtDepth(depth int) []*PlanStep {
	var result []*PlanStep

	// Calculate depth for each step
	stepDepths := p.calculateStepDepths()

	for _, step := range p.Steps {
		if stepDepths[step.ID] == depth {
			result = append(result, step)
		}
	}

	return result
}

// calculateStepDepths calculates the execution depth for each step.
func (p *QueryPlan) calculateStepDepths() map[string]int {
	depths := make(map[string]int)

	// Initialize depth 0 for steps with no dependencies
	for _, step := range p.Steps {
		if len(step.Dependencies) == 0 {
			depths[step.ID] = 0
		}
	}

	// Iteratively calculate depths
	changed := true
	for changed {
		changed = false
		for _, step := range p.Steps {
			if depths[step.ID] > 0 {
				continue // Already calculated
			}

			// Find max depth of dependencies
			maxDepDepth := -1
			for _, depID := range step.Dependencies {
				if depDepth, ok := depths[depID]; ok {
					if depDepth > maxDepDepth {
						maxDepDepth = depDepth
					}
				}
			}

			if maxDepDepth >= 0 {
				depths[step.ID] = maxDepDepth + 1
				changed = true
			}
		}
	}

	return depths
}

// MaxDepth returns the maximum depth of the plan.
func (p *QueryPlan) MaxDepth() int {
	depths := p.calculateStepDepths()
	maxDepth := 0
	for _, depth := range depths {
		if depth > maxDepth {
			maxDepth = depth
		}
	}
	return maxDepth
}
