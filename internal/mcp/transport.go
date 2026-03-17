package mcp

import (
	"encoding/json"
	"fmt"
	"io"
)

// Request represents a JSON-RPC 2.0 request.
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
	ID      *RequestID      `json:"id,omitempty"`
}

// Response represents a JSON-RPC 2.0 response.
type Response struct {
	JSONRPC string     `json:"jsonrpc"`
	Result  any        `json:"result,omitempty"`
	Error   *Error     `json:"error,omitempty"`
	ID      *RequestID `json:"id,omitempty"`
}

// RequestID represents a JSON-RPC request ID which can be
// a string, number, or null.
type RequestID struct {
	value any
}

// NewRequestID creates a new RequestID from a string or number.
func NewRequestID(id any) *RequestID {
	if id == nil {
		return nil
	}
	return &RequestID{value: id}
}

// IsNull returns true if the ID is null.
func (r *RequestID) IsNull() bool {
	return r == nil || r.value == nil
}

// String returns the string representation of the ID.
func (r *RequestID) String() string {
	if r == nil || r.value == nil {
		return "null"
	}
	switch v := r.value.(type) {
	case string:
		return v
	case float64:
		// JSON numbers decode as float64
		return fmt.Sprintf("%g", v)
	case int:
		return fmt.Sprintf("%d", v)
	case int64:
		return fmt.Sprintf("%d", v)
	case json.Number:
		return string(v)
	default:
		return ""
	}
}

// Value returns the underlying value (string or number).
func (r *RequestID) Value() any {
	if r == nil {
		return nil
	}
	return r.value
}

// MarshalJSON implements json.Marshaler.
func (r *RequestID) MarshalJSON() ([]byte, error) {
	if r == nil || r.value == nil {
		return []byte("null"), nil
	}
	return json.Marshal(r.value)
}

// UnmarshalJSON implements json.Unmarshaler.
func (r *RequestID) UnmarshalJSON(data []byte) error {
	// Check for null first
	if string(data) == "null" {
		r.value = nil
		return nil
	}

	// Try string first
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		r.value = s
		return nil
	}

	// Try number (as json.Number to preserve precision)
	var n json.Number
	if err := json.Unmarshal(data, &n); err == nil {
		r.value = n
		return nil
	}

	// Unknown type, set to nil
	r.value = nil
	return nil
}

// IsValid returns true if the request is a valid JSON-RPC 2.0 request.
func (r *Request) IsValid() bool {
	return r.JSONRPC == "2.0" && r.Method != ""
}

// IsNotification returns true if this is a notification (no ID).
func (r *Request) IsNotification() bool {
	return r.ID == nil || r.ID.IsNull()
}

// NewResponse creates a successful response for this request.
func (r *Request) NewResponse(result any) *Response {
	return &Response{
		JSONRPC: "2.0",
		Result:  result,
		ID:      r.ID,
	}
}

// NewErrorResponse creates an error response for this request.
func (r *Request) NewErrorResponse(err *Error) *Response {
	return &Response{
		JSONRPC: "2.0",
		Error:   err,
		ID:      r.ID,
	}
}

// ParseRequest parses a single JSON-RPC request from bytes.
func ParseRequest(data []byte) (*Request, error) {
	var req Request
	if err := json.Unmarshal(data, &req); err != nil {
		return nil, err
	}

	// Validate jsonrpc version
	if req.JSONRPC != "2.0" {
		return nil, ErrInvalidReq
	}

	// Validate method is present
	if req.Method == "" {
		return nil, ErrInvalidReq
	}

	return &req, nil
}

// ParseRequestBatch parses a batch of JSON-RPC requests from bytes.
// Returns nil, nil if the data represents a single request, not a batch.
func ParseRequestBatch(data []byte) ([]*Request, error) {
	// Check if it's an array
	if len(data) == 0 || data[0] != '[' {
		return nil, nil
	}

	var reqs []*Request
	if err := json.Unmarshal(data, &reqs); err != nil {
		return nil, err
	}

	// Validate each request
	for _, req := range reqs {
		if req == nil {
			continue
		}
		if req.JSONRPC != "2.0" {
			return nil, ErrInvalidReq
		}
		if req.Method == "" {
			return nil, ErrInvalidReq
		}
	}

	return reqs, nil
}

// ParseRequestFromReader parses a JSON-RPC request from an io.Reader.
func ParseRequestFromReader(r io.Reader) (*Request, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return ParseRequest(data)
}

// ParseRequestBatchFromReader parses a batch of JSON-RPC requests from an io.Reader.
func ParseRequestBatchFromReader(r io.Reader) ([]*Request, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return ParseRequestBatch(data)
}

// SerializeResponse serializes a JSON-RPC response to bytes.
func SerializeResponse(resp *Response) ([]byte, error) {
	return json.Marshal(resp)
}

// SerializeResponseBatch serializes a batch of JSON-RPC responses to bytes.
func SerializeResponseBatch(resps []*Response) ([]byte, error) {
	return json.Marshal(resps)
}

// NewSuccessResponse creates a successful JSON-RPC response.
func NewSuccessResponse(id *RequestID, result any) *Response {
	return &Response{
		JSONRPC: "2.0",
		Result:  result,
		ID:      id,
	}
}

// NewErrorResponse creates an error JSON-RPC response.
func NewErrorResponse(id *RequestID, err *Error) *Response {
	return &Response{
		JSONRPC: "2.0",
		Error:   err,
		ID:      id,
	}
}
