package rest2gql

import (
	"fmt"
	"strconv"
	"strings"
)

// PaginationStyle represents different pagination approaches.
type PaginationStyle string

const (
	// OffsetPagination uses page/limit or offset/limit parameters.
	OffsetPagination PaginationStyle = "offset"

	// CursorPagination uses before/after cursors.
	CursorPagination PaginationStyle = "cursor"

	// RelayPagination uses first/after, last/before with connection types.
	RelayPagination PaginationStyle = "relay"
)

// PaginationParams holds pagination parameters from any style.
type PaginationParams struct {
	Style PaginationStyle

	// Offset style
	Page  *int // 1-based page number
	Limit int  // Items per page

	// Cursor style
	After  string // Cursor for items after
	Before string // Cursor for items before
	First  *int   // Number of items to fetch after cursor
	Last   *int   // Number of items to fetch before cursor

	// Common
	Offset int // Calculated offset (for internal use)
	Total  int // Total count (if available)
}

// PaginationTranslator converts between pagination styles.
type PaginationTranslator struct {
	defaultLimit int
	maxLimit     int
}

// PaginationTranslatorOptions configures the translator.
type PaginationTranslatorOptions struct {
	DefaultLimit int
	MaxLimit     int
}

// NewPaginationTranslator creates a new pagination translator.
func NewPaginationTranslator(opts PaginationTranslatorOptions) *PaginationTranslator {
	defaultLimit := opts.DefaultLimit
	if defaultLimit == 0 {
		defaultLimit = 20
	}

	maxLimit := opts.MaxLimit
	if maxLimit == 0 {
		maxLimit = 100
	}

	return &PaginationTranslator{
		defaultLimit: defaultLimit,
		maxLimit:     maxLimit,
	}
}

// DetectStyle detects pagination style from GraphQL arguments.
func (pt *PaginationTranslator) DetectStyle(args map[string]interface{}) PaginationStyle {
	// Check for Relay style first
	if _, hasFirst := args["first"]; hasFirst {
		return RelayPagination
	}
	if _, hasLast := args["last"]; hasLast {
		return RelayPagination
	}

	// Check for cursor style
	if _, hasAfter := args["after"]; hasAfter {
		return CursorPagination
	}
	if _, hasBefore := args["before"]; hasBefore {
		return CursorPagination
	}

	// Default to offset
	return OffsetPagination
}

// FromGraphQLArgs extracts pagination params from GraphQL arguments.
func (pt *PaginationTranslator) FromGraphQLArgs(args map[string]interface{}) *PaginationParams {
	style := pt.DetectStyle(args)

	switch style {
	case RelayPagination:
		return pt.parseRelayArgs(args)
	case CursorPagination:
		return pt.parseCursorArgs(args)
	default:
		return pt.parseOffsetArgs(args)
	}
}

// parseRelayArgs parses Relay-style connection arguments.
func (pt *PaginationTranslator) parseRelayArgs(args map[string]interface{}) *PaginationParams {
	params := &PaginationParams{
		Style: RelayPagination,
		Limit: pt.defaultLimit,
	}

	if first, ok := args["first"].(int); ok {
		params.First = &first
		params.Limit = pt.clampLimit(first)
	}

	if last, ok := args["last"].(int); ok {
		params.Last = &last
		params.Limit = pt.clampLimit(last)
	}

	if after, ok := args["after"].(string); ok {
		params.After = after
	}

	if before, ok := args["before"].(string); ok {
		params.Before = before
	}

	return params
}

// parseCursorArgs parses cursor-based arguments.
func (pt *PaginationTranslator) parseCursorArgs(args map[string]interface{}) *PaginationParams {
	params := &PaginationParams{
		Style: CursorPagination,
		Limit: pt.defaultLimit,
	}

	if after, ok := args["after"].(string); ok {
		params.After = after
	}

	if before, ok := args["before"].(string); ok {
		params.Before = before
	}

	if limit, ok := args["limit"].(int); ok {
		params.Limit = pt.clampLimit(limit)
	}

	if first, ok := args["first"].(int); ok {
		params.First = &first
		params.Limit = pt.clampLimit(first)
	}

	return params
}

// parseOffsetArgs parses offset/limit arguments.
func (pt *PaginationTranslator) parseOffsetArgs(args map[string]interface{}) *PaginationParams {
	params := &PaginationParams{
		Style: OffsetPagination,
		Limit: pt.defaultLimit,
		Page:  nil,
	}

	// Check for offset directly
	if offset, ok := args["offset"].(int); ok {
		params.Offset = offset
	} else if offsetFloat, ok := args["offset"].(float64); ok {
		params.Offset = int(offsetFloat)
	}

	// Check for page
	if page, ok := args["page"].(int); ok {
		params.Page = &page
		if page > 0 {
			params.Offset = (page - 1) * params.Limit
		}
	} else if pageFloat, ok := args["page"].(float64); ok {
		pageInt := int(pageFloat)
		params.Page = &pageInt
		if pageInt > 0 {
			params.Offset = (pageInt - 1) * params.Limit
		}
	}

	// Check for limit
	if limit, ok := args["limit"].(int); ok {
		params.Limit = pt.clampLimit(limit)
		// Recalculate offset if page was set
		if params.Page != nil && *params.Page > 0 {
			params.Offset = (*params.Page - 1) * params.Limit
		}
	} else if limitFloat, ok := args["limit"].(float64); ok {
		params.Limit = pt.clampLimit(int(limitFloat))
		if params.Page != nil && *params.Page > 0 {
			params.Offset = (*params.Page - 1) * params.Limit
		}
	}

	return params
}

// ToRESTParams converts pagination params to REST query parameters.
func (pt *PaginationTranslator) ToRESTParams(params *PaginationParams, style PaginationStyle) map[string]string {
	result := make(map[string]string)

	switch style {
	case OffsetPagination:
		result["offset"] = strconv.Itoa(params.Offset)
		result["limit"] = strconv.Itoa(params.Limit)
		if params.Page != nil {
			result["page"] = strconv.Itoa(*params.Page)
		}

	case CursorPagination:
		if params.After != "" {
			result["cursor"] = params.After
		} else if params.Before != "" {
			result["cursor"] = params.Before
		}
		result["limit"] = strconv.Itoa(params.Limit)

	case RelayPagination:
		// Map Relay to cursor-based REST
		if params.After != "" {
			result["after"] = params.After
		}
		if params.Before != "" {
			result["before"] = params.Before
		}
		result["limit"] = strconv.Itoa(params.Limit)
	}

	return result
}

// ToGraphQLArgs converts REST pagination to GraphQL arguments.
func (pt *PaginationTranslator) ToGraphQLArgs(restParams map[string]string) map[string]interface{} {
	args := make(map[string]interface{})

	// Detect REST style
	hasPage := false
	hasOffset := false
	hasCursor := false

	if _, ok := restParams["page"]; ok {
		hasPage = true
	}
	if _, ok := restParams["offset"]; ok {
		hasOffset = true
	}
	if _, ok := restParams["cursor"]; ok {
		hasCursor = true
	}

	if hasPage || hasOffset {
		// Offset style
		if pageStr, ok := restParams["page"]; ok {
			if page, err := strconv.Atoi(pageStr); err == nil {
				args["page"] = page
			}
		}
		if offsetStr, ok := restParams["offset"]; ok {
			if offset, err := strconv.Atoi(offsetStr); err == nil {
				args["offset"] = offset
			}
		}
		if limitStr, ok := restParams["limit"]; ok {
			if limit, err := strconv.Atoi(limitStr); err == nil {
				args["limit"] = limit
			}
		}
	} else if hasCursor {
		// Cursor style
		if cursor, ok := restParams["cursor"]; ok {
			args["after"] = cursor
		}
		if limitStr, ok := restParams["limit"]; ok {
			if limit, err := strconv.Atoi(limitStr); err == nil {
				args["first"] = limit
			}
		}
	}

	return args
}

// BuildConnection builds a Relay-style connection from a list of items.
func (pt *PaginationTranslator) BuildConnection(items []interface{}, params *PaginationParams, hasNextPage bool) *Connection {
	edges := make([]Edge, len(items))
	for i, item := range items {
		edges[i] = Edge{
			Node:   item,
			Cursor: encodeCursor(i, params),
		}
	}

	var startCursor, endCursor string
	if len(edges) > 0 {
		startCursor = edges[0].Cursor
		endCursor = edges[len(edges)-1].Cursor
	}

	return &Connection{
		Edges: edges,
		PageInfo: PageInfo{
			HasNextPage:     hasNextPage,
			HasPreviousPage: params.After != "" || params.Offset > 0,
			StartCursor:     startCursor,
			EndCursor:       endCursor,
		},
	}
}

// Connection represents a Relay-style connection.
type Connection struct {
	Edges    []Edge   `json:"edges"`
	PageInfo PageInfo `json:"pageInfo"`
	Total    int      `json:"total,omitempty"`
}

// Edge represents an edge in a Relay connection.
type Edge struct {
	Node   interface{} `json:"node"`
	Cursor string      `json:"cursor"`
}

// PageInfo represents pagination metadata.
type PageInfo struct {
	HasNextPage     bool   `json:"hasNextPage"`
	HasPreviousPage bool   `json:"hasPreviousPage"`
	StartCursor     string `json:"startCursor,omitempty"`
	EndCursor       string `json:"endCursor,omitempty"`
}

// encodeCursor creates a cursor from an offset.
func encodeCursor(index int, params *PaginationParams) string {
	// Simple cursor encoding based on offset
	offset := params.Offset + index
	return base64Encode(fmt.Sprintf("cursor:%d", offset))
}

// DecodeCursor extracts offset from a cursor.
func DecodeCursor(cursor string) (int, error) {
	decoded, err := base64Decode(cursor)
	if err != nil {
		return 0, err
	}

	// Parse "cursor:offset" format
	parts := strings.Split(decoded, ":")
	if len(parts) != 2 || parts[0] != "cursor" {
		return 0, fmt.Errorf("invalid cursor format")
	}

	offset, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, fmt.Errorf("invalid cursor offset: %w", err)
	}

	return offset, nil
}

// clampLimit ensures limit is within bounds.
func (pt *PaginationTranslator) clampLimit(limit int) int {
	if limit <= 0 {
		return pt.defaultLimit
	}
	if limit > pt.maxLimit {
		return pt.maxLimit
	}
	return limit
}

// base64Encode encodes a string to base64.
func base64Encode(s string) string {
	// Simple encoding - in production use actual base64
	return "Y3Vyc29yOg==" + s
}

// base64Decode decodes a base64 string.
func base64Decode(s string) (string, error) {
	// Simple decoding - in production use actual base64
	if len(s) < 12 {
		return "", fmt.Errorf("invalid cursor")
	}
	return s[12:], nil
}
