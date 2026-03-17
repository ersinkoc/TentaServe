package schema

import (
	"testing"
)

// TestNewPlanBuilder tests plan builder creation.
func TestNewPlanBuilder(t *testing.T) {
	pb := NewPlanBuilder()

	if pb == nil {
		t.Fatal("Expected PlanBuilder to be created")
	}

	if len(pb.upstreamDeps) != 0 {
		t.Errorf("Expected 0 upstream deps initially, got %d", len(pb.upstreamDeps))
	}
}

// TestPlanBuilder_RegisterUpstreamDependency tests dependency registration.
func TestPlanBuilder_RegisterUpstreamDependency(t *testing.T) {
	pb := NewPlanBuilder()

	pb.RegisterUpstreamDependency("users", "user-service")

	if len(pb.upstreamDeps["users"]) != 1 {
		t.Errorf("Expected 1 dependency, got %d", len(pb.upstreamDeps["users"]))
	}

	if pb.upstreamDeps["users"][0] != "user-service" {
		t.Errorf("Expected 'user-service', got %s", pb.upstreamDeps["users"][0])
	}
}

// TestPlanBuilder_RegisterUpstreamDependency_Duplicate tests duplicate registration.
func TestPlanBuilder_RegisterUpstreamDependency_Duplicate(t *testing.T) {
	pb := NewPlanBuilder()

	pb.RegisterUpstreamDependency("users", "user-service")
	pb.RegisterUpstreamDependency("users", "user-service") // Duplicate

	if len(pb.upstreamDeps["users"]) != 1 {
		t.Errorf("Expected 1 dependency after duplicate registration, got %d", len(pb.upstreamDeps["users"]))
	}
}

// TestPlanBuilder_BuildPlan_SingleUpstream tests building plan with single upstream.
func TestPlanBuilder_BuildPlan_SingleUpstream(t *testing.T) {
	pb := NewPlanBuilder()

	fields := []PlanField{
		{
			Path:         "users",
			UpstreamName: "user-service",
		},
	}

	plan, err := pb.BuildPlan("query", fields)
	if err != nil {
		t.Fatalf("BuildPlan failed: %v", err)
	}

	if plan.StepCount() != 1 {
		t.Errorf("Expected 1 step, got %d", plan.StepCount())
	}

	step := plan.Steps[0]
	if step.UpstreamName != "user-service" {
		t.Errorf("Expected upstream 'user-service', got %s", step.UpstreamName)
	}

	if step.FieldPath != "users" {
		t.Errorf("Expected field path 'users', got %s", step.FieldPath)
	}

	if len(step.Dependencies) != 0 {
		t.Errorf("Expected 0 dependencies, got %d", len(step.Dependencies))
	}
}

// TestPlanBuilder_BuildPlan_MultipleFields tests building plan with multiple fields.
func TestPlanBuilder_BuildPlan_MultipleFields(t *testing.T) {
	pb := NewPlanBuilder()

	fields := []PlanField{
		{Path: "users", UpstreamName: "user-service"},
		{Path: "posts", UpstreamName: "post-service"},
	}

	plan, err := pb.BuildPlan("query", fields)
	if err != nil {
		t.Fatalf("BuildPlan failed: %v", err)
	}

	if plan.StepCount() != 2 {
		t.Errorf("Expected 2 steps, got %d", plan.StepCount())
	}

	// Steps at same depth should be parallel
	for _, step := range plan.Steps {
		if len(step.Dependencies) != 0 {
			t.Errorf("Expected 0 dependencies for parallel steps, got %d", len(step.Dependencies))
		}
		if !step.Parallel {
			t.Error("Expected steps to be parallel")
		}
	}
}

// TestPlanBuilder_BuildPlan_WithDependencies tests building plan with field dependencies.
func TestPlanBuilder_BuildPlan_WithDependencies(t *testing.T) {
	pb := NewPlanBuilder()

	fields := []PlanField{
		{
			Path:         "user",
			UpstreamName: "user-service",
		},
		{
			Path:         "user.posts",
			UpstreamName: "post-service",
			Dependencies: []string{"user"},
		},
	}

	plan, err := pb.BuildPlan("query", fields)
	if err != nil {
		t.Fatalf("BuildPlan failed: %v", err)
	}

	if plan.StepCount() != 2 {
		t.Errorf("Expected 2 steps, got %d", plan.StepCount())
	}

	// Find user step and posts step
	var userStep, postsStep *PlanStep
	for _, step := range plan.Steps {
		if step.UpstreamName == "user-service" {
			userStep = step
		}
		if step.UpstreamName == "post-service" {
			postsStep = step
		}
	}

	if userStep == nil {
		t.Fatal("Expected user step")
	}
	if postsStep == nil {
		t.Fatal("Expected posts step")
	}

	// Posts step should depend on user step
	if len(postsStep.Dependencies) != 1 {
		t.Errorf("Expected 1 dependency for posts step, got %d", len(postsStep.Dependencies))
	}

	if postsStep.Dependencies[0] != userStep.ID {
		t.Errorf("Expected posts to depend on user step (%s), got %s", userStep.ID, postsStep.Dependencies[0])
	}

	// User step should have no dependencies
	if len(userStep.Dependencies) != 0 {
		t.Errorf("Expected 0 dependencies for user step, got %d", len(userStep.Dependencies))
	}
}

// TestPlanBuilder_BuildPlan_CycleDetection tests cycle detection.
func TestPlanBuilder_BuildPlan_CycleDetection(t *testing.T) {
	pb := NewPlanBuilder()

	fields := []PlanField{
		{
			Path:         "a",
			UpstreamName: "service-a",
			Dependencies: []string{"b"},
		},
		{
			Path:         "b",
			UpstreamName: "service-b",
			Dependencies: []string{"a"},
		},
	}

	_, err := pb.BuildPlan("query", fields)
	if err == nil {
		t.Fatal("Expected error for cyclic dependencies")
	}

	if err.Error() != "dependency resolution failed: cycle detected in query dependencies" {
		t.Errorf("Unexpected error message: %v", err)
	}
}

// TestPlanBuilder_BuildPlan_EmptyFields tests building plan with empty fields.
func TestPlanBuilder_BuildPlan_EmptyFields(t *testing.T) {
	pb := NewPlanBuilder()

	plan, err := pb.BuildPlan("query", []PlanField{})
	if err != nil {
		t.Fatalf("BuildPlan failed: %v", err)
	}

	if plan.StepCount() != 0 {
		t.Errorf("Expected 0 steps for empty fields, got %d", plan.StepCount())
	}
}

// TestPlanBuilder_BuildPlan_ThreeUpstreams tests plan with three upstreams in sequence.
func TestPlanBuilder_BuildPlan_ThreeUpstreams(t *testing.T) {
	pb := NewPlanBuilder()

	fields := []PlanField{
		{
			Path:         "user",
			UpstreamName: "user-service",
		},
		{
			Path:         "user.posts",
			UpstreamName: "post-service",
			Dependencies: []string{"user"},
		},
		{
			Path:         "user.posts.comments",
			UpstreamName: "comment-service",
			Dependencies: []string{"user.posts"},
		},
	}

	plan, err := pb.BuildPlan("query", fields)
	if err != nil {
		t.Fatalf("BuildPlan failed: %v", err)
	}

	if plan.StepCount() != 3 {
		t.Errorf("Expected 3 steps, got %d", plan.StepCount())
	}

	// Check depths
	if plan.MaxDepth() != 2 {
		t.Errorf("Expected max depth 2, got %d", plan.MaxDepth())
	}

	// Check sequential ordering (each depends on previous)
	depth0Steps := plan.GetStepsAtDepth(0)
	depth1Steps := plan.GetStepsAtDepth(1)
	depth2Steps := plan.GetStepsAtDepth(2)

	if len(depth0Steps) != 1 {
		t.Errorf("Expected 1 step at depth 0, got %d", len(depth0Steps))
	}
	if len(depth1Steps) != 1 {
		t.Errorf("Expected 1 step at depth 1, got %d", len(depth1Steps))
	}
	if len(depth2Steps) != 1 {
		t.Errorf("Expected 1 step at depth 2, got %d", len(depth2Steps))
	}

	if depth0Steps[0].UpstreamName != "user-service" {
		t.Errorf("Expected user-service at depth 0, got %s", depth0Steps[0].UpstreamName)
	}
}

// TestPlanBuilder_BuildPlan_ParallelAtSameDepth tests parallel steps at same depth.
func TestPlanBuilder_BuildPlan_ParallelAtSameDepth(t *testing.T) {
	pb := NewPlanBuilder()

	fields := []PlanField{
		{
			Path:         "user",
			UpstreamName: "user-service",
		},
		{
			Path:         "products",
			UpstreamName: "product-service",
		},
		{
			Path:         "user.orders",
			UpstreamName: "order-service",
			Dependencies: []string{"user"},
		},
	}

	plan, err := pb.BuildPlan("query", fields)
	if err != nil {
		t.Fatalf("BuildPlan failed: %v", err)
	}

	if plan.StepCount() != 3 {
		t.Errorf("Expected 3 steps, got %d", plan.StepCount())
	}

	// Depth 0 should have user and products (parallel)
	depth0Steps := plan.GetStepsAtDepth(0)
	if len(depth0Steps) != 2 {
		t.Errorf("Expected 2 steps at depth 0, got %d", len(depth0Steps))
	}

	// Both should be parallel
	for _, step := range depth0Steps {
		if !step.Parallel {
			t.Error("Expected steps at depth 0 to be parallel")
		}
	}

	// Depth 1 should have orders (depends on user)
	depth1Steps := plan.GetStepsAtDepth(1)
	if len(depth1Steps) != 1 {
		t.Errorf("Expected 1 step at depth 1, got %d", len(depth1Steps))
	}
	if depth1Steps[0].UpstreamName != "order-service" {
		t.Errorf("Expected order-service at depth 1, got %s", depth1Steps[0].UpstreamName)
	}
}

// TestQueryPlan_GetStep tests retrieving steps by ID.
func TestQueryPlan_GetStep(t *testing.T) {
	pb := NewPlanBuilder()

	fields := []PlanField{
		{Path: "users", UpstreamName: "user-service"},
	}

	plan, err := pb.BuildPlan("query", fields)
	if err != nil {
		t.Fatalf("BuildPlan failed: %v", err)
	}

	step := plan.Steps[0]

	// Get step by ID
	retrieved := plan.GetStep(step.ID)
	if retrieved != step {
		t.Error("Expected GetStep to return the same step")
	}

	// Get non-existent step
	retrieved = plan.GetStep("non-existent")
	if retrieved != nil {
		t.Error("Expected GetStep to return nil for non-existent ID")
	}
}

// TestQueryPlan_GetStepsAtDepth tests retrieving steps by depth.
func TestQueryPlan_GetStepsAtDepth(t *testing.T) {
	pb := NewPlanBuilder()

	fields := []PlanField{
		{Path: "users", UpstreamName: "user-service"},
	}

	plan, err := pb.BuildPlan("query", fields)
	if err != nil {
		t.Fatalf("BuildPlan failed: %v", err)
	}

	steps := plan.GetStepsAtDepth(0)
	if len(steps) != 1 {
		t.Errorf("Expected 1 step at depth 0, got %d", len(steps))
	}

	steps = plan.GetStepsAtDepth(1)
	if len(steps) != 0 {
		t.Errorf("Expected 0 steps at depth 1, got %d", len(steps))
	}
}

// TestQueryPlan_MaxDepth tests max depth calculation.
func TestQueryPlan_MaxDepth(t *testing.T) {
	pb := NewPlanBuilder()

	// Empty plan
	plan := NewQueryPlan()
	if plan.MaxDepth() != 0 {
		t.Errorf("Expected max depth 0 for empty plan, got %d", plan.MaxDepth())
	}

	// Plan with single step
	fields := []PlanField{
		{Path: "users", UpstreamName: "user-service"},
	}
	plan, _ = pb.BuildPlan("query", fields)
	if plan.MaxDepth() != 0 {
		t.Errorf("Expected max depth 0 for single step plan, got %d", plan.MaxDepth())
	}

	// Plan with sequential dependencies
	fields = []PlanField{
		{Path: "user", UpstreamName: "user-service"},
		{Path: "user.posts", UpstreamName: "post-service", Dependencies: []string{"user"}},
	}
	plan, _ = pb.BuildPlan("query", fields)
	if plan.MaxDepth() != 1 {
		t.Errorf("Expected max depth 1, got %d", plan.MaxDepth())
	}
}

// TestPlanBuilder_inferOperationType tests operation type inference.
func TestPlanBuilder_inferOperationType(t *testing.T) {
	pb := NewPlanBuilder()

	// Query
	fields := []PlanField{{Path: "users", UpstreamName: "user-service"}}
	plan, _ := pb.BuildPlan("query", fields)
	if plan.Steps[0].OperationType != "query" {
		t.Errorf("Expected 'query', got %s", plan.Steps[0].OperationType)
	}

	// Mutation
	fields = []PlanField{{Path: "createUser", UpstreamName: "user-service"}}
	plan, _ = pb.BuildPlan("mutation", fields)
	if plan.Steps[0].OperationType != "mutation" {
		t.Errorf("Expected 'mutation', got %s", plan.Steps[0].OperationType)
	}

	// Update mutation
	fields = []PlanField{{Path: "updateUser", UpstreamName: "user-service"}}
	plan, _ = pb.BuildPlan("mutation", fields)
	if plan.Steps[0].OperationType != "mutation" {
		t.Errorf("Expected 'mutation' for update, got %s", plan.Steps[0].OperationType)
	}

	// Delete mutation
	fields = []PlanField{{Path: "deleteUser", UpstreamName: "user-service"}}
	plan, _ = pb.BuildPlan("mutation", fields)
	if plan.Steps[0].OperationType != "mutation" {
		t.Errorf("Expected 'mutation' for delete, got %s", plan.Steps[0].OperationType)
	}
}

// TestPlanBuilder_BuildPlan_WithDataBindings tests data extraction bindings.
func TestPlanBuilder_BuildPlan_WithDataBindings(t *testing.T) {
	pb := NewPlanBuilder()

	fields := []PlanField{
		{
			Path:         "user",
			UpstreamName: "user-service",
			DataBindings: map[string]string{"userId": "id"},
		},
	}

	plan, err := pb.BuildPlan("query", fields)
	if err != nil {
		t.Fatalf("BuildPlan failed: %v", err)
	}

	step := plan.Steps[0]
	if len(step.DataExtraction) != 1 {
		t.Errorf("Expected 1 data binding, got %d", len(step.DataExtraction))
	}

	if step.DataExtraction["userId"] != "id" {
		t.Errorf("Expected userId->id binding, got %s", step.DataExtraction["userId"])
	}
}
