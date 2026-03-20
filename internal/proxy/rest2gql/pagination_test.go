package rest2gql

import (
	"testing"
)

// TestNewPaginationTranslator tests translator creation.
func TestNewPaginationTranslator(t *testing.T) {
	pt := NewPaginationTranslator(PaginationTranslatorOptions{
		DefaultLimit: 20,
		MaxLimit:     100,
	})

	if pt == nil {
		t.Fatal("Expected PaginationTranslator to be created")
	}

	if pt.defaultLimit != 20 {
		t.Errorf("Expected defaultLimit 20, got %d", pt.defaultLimit)
	}

	if pt.maxLimit != 100 {
		t.Errorf("Expected maxLimit 100, got %d", pt.maxLimit)
	}
}

// TestNewPaginationTranslator_Defaults tests default values.
func TestNewPaginationTranslator_Defaults(t *testing.T) {
	pt := NewPaginationTranslator(PaginationTranslatorOptions{})

	if pt.defaultLimit != 20 {
		t.Errorf("Expected default defaultLimit 20, got %d", pt.defaultLimit)
	}

	if pt.maxLimit != 100 {
		t.Errorf("Expected default maxLimit 100, got %d", pt.maxLimit)
	}
}

// TestPaginationTranslator_DetectStyle tests style detection.
func TestPaginationTranslator_DetectStyle(t *testing.T) {
	pt := NewPaginationTranslator(PaginationTranslatorOptions{})

	tests := []struct {
		name     string
		args     map[string]interface{}
		expected PaginationStyle
	}{
		{
			name:     "offset style with page",
			args:     map[string]interface{}{"page": 1, "limit": 10},
			expected: OffsetPagination,
		},
		{
			name:     "offset style with offset",
			args:     map[string]interface{}{"offset": 0, "limit": 10},
			expected: OffsetPagination,
		},
		{
			name:     "cursor style with after",
			args:     map[string]interface{}{"after": "abc123", "limit": 10},
			expected: CursorPagination,
		},
		{
			name:     "cursor style with before",
			args:     map[string]interface{}{"before": "xyz789", "limit": 10},
			expected: CursorPagination,
		},
		{
			name:     "relay style with first",
			args:     map[string]interface{}{"first": 10},
			expected: RelayPagination,
		},
		{
			name:     "relay style with last",
			args:     map[string]interface{}{"last": 10},
			expected: RelayPagination,
		},
		{
			name:     "relay style with first and after",
			args:     map[string]interface{}{"first": 10, "after": "abc"},
			expected: RelayPagination,
		},
		{
			name:     "empty args (defaults to offset)",
			args:     map[string]interface{}{},
			expected: OffsetPagination,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			style := pt.DetectStyle(tt.args)
			if style != tt.expected {
				t.Errorf("Expected style %s, got %s", tt.expected, style)
			}
		})
	}
}

// TestPaginationTranslator_FromGraphQLArgs_Offset tests offset argument parsing.
func TestPaginationTranslator_FromGraphQLArgs_Offset(t *testing.T) {
	pt := NewPaginationTranslator(PaginationTranslatorOptions{DefaultLimit: 20})

	// Page-based
	params := pt.FromGraphQLArgs(map[string]interface{}{"page": 2, "limit": 10})

	if params.Style != OffsetPagination {
		t.Errorf("Expected offset style, got %s", params.Style)
	}

	if params.Page == nil || *params.Page != 2 {
		t.Errorf("Expected page 2, got %v", params.Page)
	}

	if params.Limit != 10 {
		t.Errorf("Expected limit 10, got %d", params.Limit)
	}

	// Offset should be (page-1) * limit = (2-1) * 10 = 10
	if params.Offset != 10 {
		t.Errorf("Expected offset 10, got %d", params.Offset)
	}

	// Direct offset
	params = pt.FromGraphQLArgs(map[string]interface{}{"offset": 50, "limit": 25})

	if params.Offset != 50 {
		t.Errorf("Expected offset 50, got %d", params.Offset)
	}

	if params.Limit != 25 {
		t.Errorf("Expected limit 25, got %d", params.Limit)
	}
}

// TestPaginationTranslator_FromGraphQLArgs_Cursor tests cursor argument parsing.
func TestPaginationTranslator_FromGraphQLArgs_Cursor(t *testing.T) {
	pt := NewPaginationTranslator(PaginationTranslatorOptions{DefaultLimit: 20})

	params := pt.FromGraphQLArgs(map[string]interface{}{"after": "abc123", "limit": 10})

	if params.Style != CursorPagination {
		t.Errorf("Expected cursor style, got %s", params.Style)
	}

	if params.After != "abc123" {
		t.Errorf("Expected after 'abc123', got %s", params.After)
	}

	if params.Limit != 10 {
		t.Errorf("Expected limit 10, got %d", params.Limit)
	}
}

// TestPaginationTranslator_FromGraphQLArgs_Relay tests Relay argument parsing.
func TestPaginationTranslator_FromGraphQLArgs_Relay(t *testing.T) {
	pt := NewPaginationTranslator(PaginationTranslatorOptions{DefaultLimit: 20})

	params := pt.FromGraphQLArgs(map[string]interface{}{"first": 10, "after": "abc123"})

	if params.Style != RelayPagination {
		t.Errorf("Expected relay style, got %s", params.Style)
	}

	if params.First == nil || *params.First != 10 {
		t.Errorf("Expected first 10, got %v", params.First)
	}

	if params.After != "abc123" {
		t.Errorf("Expected after 'abc123', got %s", params.After)
	}

	if params.Limit != 10 {
		t.Errorf("Expected limit 10, got %d", params.Limit)
	}
}

// TestPaginationTranslator_ToRESTParams_Offset tests REST param conversion (offset).
func TestPaginationTranslator_ToRESTParams_Offset(t *testing.T) {
	pt := NewPaginationTranslator(PaginationTranslatorOptions{})

	params := &PaginationParams{
		Style:  OffsetPagination,
		Offset: 20,
		Limit:  10,
		Page:   func() *int { i := 3; return &i }(),
	}

	restParams := pt.ToRESTParams(params, OffsetPagination)

	if restParams["offset"] != "20" {
		t.Errorf("Expected offset '20', got %s", restParams["offset"])
	}

	if restParams["limit"] != "10" {
		t.Errorf("Expected limit '10', got %s", restParams["limit"])
	}

	if restParams["page"] != "3" {
		t.Errorf("Expected page '3', got %s", restParams["page"])
	}
}

// TestPaginationTranslator_ToRESTParams_Cursor tests REST param conversion (cursor).
func TestPaginationTranslator_ToRESTParams_Cursor(t *testing.T) {
	pt := NewPaginationTranslator(PaginationTranslatorOptions{})

	params := &PaginationParams{
		Style:  CursorPagination,
		After:  "abc123",
		Limit:  10,
	}

	restParams := pt.ToRESTParams(params, CursorPagination)

	if restParams["cursor"] != "abc123" {
		t.Errorf("Expected cursor 'abc123', got %s", restParams["cursor"])
	}

	if restParams["limit"] != "10" {
		t.Errorf("Expected limit '10', got %s", restParams["limit"])
	}
}

// TestPaginationTranslator_ToRESTParams_Relay tests REST param conversion (relay).
func TestPaginationTranslator_ToRESTParams_Relay(t *testing.T) {
	pt := NewPaginationTranslator(PaginationTranslatorOptions{})

	params := &PaginationParams{
		Style:  RelayPagination,
		After:  "abc123",
		Limit:  10,
	}

	restParams := pt.ToRESTParams(params, RelayPagination)

	if restParams["after"] != "abc123" {
		t.Errorf("Expected after 'abc123', got %s", restParams["after"])
	}

	if restParams["limit"] != "10" {
		t.Errorf("Expected limit '10', got %s", restParams["limit"])
	}
}

// TestPaginationTranslator_ToGraphQLArgs_Offset tests GraphQL arg conversion from REST (offset).
func TestPaginationTranslator_ToGraphQLArgs_Offset(t *testing.T) {
	pt := NewPaginationTranslator(PaginationTranslatorOptions{})

	restParams := map[string]string{
		"page":   "2",
		"offset": "20",
		"limit":  "10",
	}

	args := pt.ToGraphQLArgs(restParams)

	if args["page"] != 2 {
		t.Errorf("Expected page 2, got %v", args["page"])
	}

	if args["offset"] != 20 {
		t.Errorf("Expected offset 20, got %v", args["offset"])
	}

	if args["limit"] != 10 {
		t.Errorf("Expected limit 10, got %v", args["limit"])
	}
}

// TestPaginationTranslator_ToGraphQLArgs_Cursor tests GraphQL arg conversion from REST (cursor).
func TestPaginationTranslator_ToGraphQLArgs_Cursor(t *testing.T) {
	pt := NewPaginationTranslator(PaginationTranslatorOptions{})

	restParams := map[string]string{
		"cursor": "abc123",
		"limit":  "10",
	}

	args := pt.ToGraphQLArgs(restParams)

	if args["after"] != "abc123" {
		t.Errorf("Expected after 'abc123', got %v", args["after"])
	}

	if args["first"] != 10 {
		t.Errorf("Expected first 10, got %v", args["first"])
	}
}

// TestPaginationTranslator_clampLimit tests limit clamping.
func TestPaginationTranslator_clampLimit(t *testing.T) {
	pt := NewPaginationTranslator(PaginationTranslatorOptions{
		DefaultLimit: 20,
		MaxLimit:     100,
	})

	tests := []struct {
		input    int
		expected int
	}{
		{0, 20},   // Zero -> default
		{-1, 20},  // Negative -> default
		{5, 5},    // Valid -> unchanged
		{50, 50},  // Valid -> unchanged
		{100, 100}, // At max -> unchanged
		{150, 100}, // Over max -> clamped
	}

	for _, tt := range tests {
		result := pt.clampLimit(tt.input)
		if result != tt.expected {
			t.Errorf("clampLimit(%d) = %d, want %d", tt.input, result, tt.expected)
		}
	}
}

// TestPaginationTranslator_E2E_OffsetToRelay tests end-to-end offset to relay.
func TestPaginationTranslator_E2E_OffsetToRelay(t *testing.T) {
	pt := NewPaginationTranslator(PaginationTranslatorOptions{DefaultLimit: 20, MaxLimit: 100})

	// GraphQL: users(page: 2, limit: 10)
	graphqlArgs := map[string]interface{}{"page": 2, "limit": 10}

	// Parse GraphQL args
	params := pt.FromGraphQLArgs(graphqlArgs)

	if params.Style != OffsetPagination {
		t.Fatalf("Expected offset style")
	}

	// Convert to REST params
	restParams := pt.ToRESTParams(params, OffsetPagination)

	if restParams["offset"] != "10" {
		t.Errorf("Expected offset '10', got %s", restParams["offset"])
	}

	// Simulate REST response, build Relay connection
	items := []interface{}{"user1", "user2", "user3"}
	connection := pt.BuildConnection(items, params, true)

	if len(connection.Edges) != 3 {
		t.Errorf("Expected 3 edges, got %d", len(connection.Edges))
	}

	if !connection.PageInfo.HasNextPage {
		t.Error("Expected HasNextPage to be true")
	}
}

// TestDecodeCursor tests cursor decoding.
func TestDecodeCursor(t *testing.T) {
	// First encode
	cursor := encodeCursor(42, &PaginationParams{Offset: 0})

	// Then decode
	offset, err := DecodeCursor(cursor)
	if err != nil {
		t.Fatalf("DecodeCursor failed: %v", err)
	}

	if offset != 42 {
		t.Errorf("Expected offset 42, got %d", offset)
	}
}

// TestDecodeCursor_InvalidFormat tests decoding invalid cursor.
func TestDecodeCursor_InvalidFormat(t *testing.T) {
	_, err := DecodeCursor("invalid")
	if err == nil {
		t.Error("Expected error for invalid cursor")
	}
}

// TestDecodeCursor_TooShort tests decoding too short cursor.
func TestDecodeCursor_TooShort(t *testing.T) {
	_, err := DecodeCursor("abc")
	if err == nil {
		t.Error("Expected error for too short cursor")
	}
}

// TestBuildConnection tests connection building.
func TestBuildConnection(t *testing.T) {
	pt := NewPaginationTranslator(PaginationTranslatorOptions{})

	items := []interface{}{
		map[string]string{"id": "1", "name": "User 1"},
		map[string]string{"id": "2", "name": "User 2"},
		map[string]string{"id": "3", "name": "User 3"},
	}

	params := &PaginationParams{
		Style:  RelayPagination,
		After:  "",
		Offset: 0,
		Limit:  3,
	}

	connection := pt.BuildConnection(items, params, true)

	if len(connection.Edges) != 3 {
		t.Errorf("Expected 3 edges, got %d", len(connection.Edges))
	}

	if !connection.PageInfo.HasNextPage {
		t.Error("Expected HasNextPage to be true")
	}

	if connection.PageInfo.HasPreviousPage {
		t.Error("Expected HasPreviousPage to be false")
	}

	if connection.PageInfo.StartCursor == "" {
		t.Error("Expected StartCursor to be set")
	}

	if connection.PageInfo.EndCursor == "" {
		t.Error("Expected EndCursor to be set")
	}
}

// TestBuildConnection_WithAfter tests connection with after cursor.
func TestBuildConnection_WithAfter(t *testing.T) {
	pt := NewPaginationTranslator(PaginationTranslatorOptions{})

	items := []interface{}{"user1", "user2"}

	params := &PaginationParams{
		Style:  RelayPagination,
		After:  "somecursor",
		Offset: 10,
		Limit:  2,
	}

	connection := pt.BuildConnection(items, params, false)

	if !connection.PageInfo.HasPreviousPage {
		t.Error("Expected HasPreviousPage to be true when after is set")
	}

	if connection.PageInfo.HasNextPage {
		t.Error("Expected HasNextPage to be false")
	}
}

// TestPaginationTranslator_FromGraphQLArgs_OffsetFloat tests float offset args.
func TestPaginationTranslator_FromGraphQLArgs_OffsetFloat(t *testing.T) {
	pt := NewPaginationTranslator(PaginationTranslatorOptions{DefaultLimit: 20})

	// Float offset and page values (JSON numbers decode as float64)
	params := pt.FromGraphQLArgs(map[string]interface{}{
		"offset": float64(30),
		"limit":  float64(15),
		"page":   float64(3),
	})

	if params.Offset != 30 {
		t.Errorf("Expected offset 30, got %d", params.Offset)
	}
	if params.Limit != 15 {
		t.Errorf("Expected limit 15, got %d", params.Limit)
	}
	if params.Page == nil || *params.Page != 3 {
		t.Errorf("Expected page 3, got %v", params.Page)
	}
}

// TestPaginationTranslator_ToRESTParams_CursorBefore tests before cursor.
func TestPaginationTranslator_ToRESTParams_CursorBefore(t *testing.T) {
	pt := NewPaginationTranslator(PaginationTranslatorOptions{})

	params := &PaginationParams{
		Style:  CursorPagination,
		Before: "xyz789",
		Limit:  10,
	}

	restParams := pt.ToRESTParams(params, CursorPagination)
	if restParams["cursor"] != "xyz789" {
		t.Errorf("Expected cursor 'xyz789', got %s", restParams["cursor"])
	}
}

// TestPaginationTranslator_Relay_LastBefore tests last/before relay args.
func TestPaginationTranslator_Relay_LastBefore(t *testing.T) {
	pt := NewPaginationTranslator(PaginationTranslatorOptions{DefaultLimit: 20})

	params := pt.FromGraphQLArgs(map[string]interface{}{
		"last":   10,
		"before": "cursor123",
	})

	if params.Style != RelayPagination {
		t.Errorf("Expected relay style, got %s", params.Style)
	}
	if params.Last == nil || *params.Last != 10 {
		t.Errorf("Expected last 10, got %v", params.Last)
	}
	if params.Before != "cursor123" {
		t.Errorf("Expected before 'cursor123', got %s", params.Before)
	}
}

// TestPaginationTranslator_ToRESTParams_RelayBefore tests relay with before param.
func TestPaginationTranslator_ToRESTParams_RelayBefore(t *testing.T) {
	pt := NewPaginationTranslator(PaginationTranslatorOptions{})

	params := &PaginationParams{
		Style:  RelayPagination,
		Before: "xyz789",
		Limit:  5,
	}

	restParams := pt.ToRESTParams(params, RelayPagination)
	if restParams["before"] != "xyz789" {
		t.Errorf("Expected before 'xyz789', got %s", restParams["before"])
	}
}

// TestPaginationTranslator_ToRESTParams_OffsetNoPage tests offset without page.
func TestPaginationTranslator_ToRESTParams_OffsetNoPage(t *testing.T) {
	pt := NewPaginationTranslator(PaginationTranslatorOptions{})

	params := &PaginationParams{
		Style:  OffsetPagination,
		Offset: 0,
		Limit:  20,
	}

	restParams := pt.ToRESTParams(params, OffsetPagination)
	if _, ok := restParams["page"]; ok {
		t.Error("Expected no page param when Page is nil")
	}
}

// TestBuildConnection_Empty tests empty connection.
func TestBuildConnection_Empty(t *testing.T) {
	pt := NewPaginationTranslator(PaginationTranslatorOptions{})

	items := []interface{}{}

	params := &PaginationParams{
		Style: RelayPagination,
	}

	connection := pt.BuildConnection(items, params, false)

	if len(connection.Edges) != 0 {
		t.Errorf("Expected 0 edges, got %d", len(connection.Edges))
	}

	if connection.PageInfo.StartCursor != "" {
		t.Error("Expected empty StartCursor for empty result")
	}

	if connection.PageInfo.EndCursor != "" {
		t.Error("Expected empty EndCursor for empty result")
	}
}
