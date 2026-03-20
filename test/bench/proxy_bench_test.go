package bench_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// BenchmarkPassthrough measures raw reverse-proxy passthrough performance.
// A request is forwarded to a backend httptest.Server and the response is
// written back through a minimal proxy handler.
func BenchmarkPassthrough(b *testing.B) {
	b.ReportAllocs()

	// Spin up a mock backend that returns a static JSON payload.
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":1,"name":"bench","status":"ok"}`))
	}))
	defer backend.Close()

	// Build a lightweight reverse-proxy handler that forwards to the backend.
	transport := &http.Transport{}
	client := &http.Client{Transport: transport}

	proxyHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxyReq, err := http.NewRequestWithContext(r.Context(), r.Method, backend.URL+r.URL.Path, r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		for k, vv := range r.Header {
			for _, v := range vv {
				proxyReq.Header.Add(k, v)
			}
		}
		resp, err := client.Do(proxyReq)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		for k, vv := range resp.Header {
			for _, v := range vv {
				w.Header().Add(k, v)
			}
		}
		w.WriteHeader(resp.StatusCode)
		io.Copy(w, resp.Body)
	})

	proxy := httptest.NewServer(proxyHandler)
	defer proxy.Close()

	req, _ := http.NewRequest(http.MethodGet, proxy.URL+"/api/items", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			b.Fatal(err)
		}
		io.ReadAll(resp.Body)
		resp.Body.Close()
	}
}

// BenchmarkREST2GQL measures the cost of translating an inbound REST request
// into a GraphQL query, executing it against a mock GraphQL backend, and
// returning the JSON response.
func BenchmarkREST2GQL(b *testing.B) {
	b.ReportAllocs()

	// Mock GraphQL backend that accepts a POST with a JSON body containing
	// a "query" field and returns canned data.
	gqlBackend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		r.Body.Close()

		var gqlReq struct {
			Query string `json:"query"`
		}
		json.Unmarshal(body, &gqlReq)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// Return a canned GraphQL response wrapping the queried field.
		w.Write([]byte(`{"data":{"users":[{"id":"1","name":"Alice"},{"id":"2","name":"Bob"}]}}`))
	}))
	defer gqlBackend.Close()

	// REST-to-GraphQL proxy: accept GET /api/users, translate to
	// { query: "{ users { id name } }" }, post to the GraphQL backend,
	// unwrap and return the REST response.
	gqlClient := &http.Client{}

	rest2gqlHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Build a GraphQL query from the REST path.
		gqlQuery := `{"query":"{ users { id name } }"}`

		req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, gqlBackend.URL, strings.NewReader(gqlQuery))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := gqlClient.Do(req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		// Unwrap and forward
		var gqlResp struct {
			Data json.RawMessage `json:"data"`
		}
		json.NewDecoder(resp.Body).Decode(&gqlResp)

		// Extract the target field
		var dataMap map[string]json.RawMessage
		json.Unmarshal(gqlResp.Data, &dataMap)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(dataMap["users"])
	})

	proxy := httptest.NewServer(rest2gqlHandler)
	defer proxy.Close()

	req, _ := http.NewRequest(http.MethodGet, proxy.URL+"/api/users", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			b.Fatal(err)
		}
		io.ReadAll(resp.Body)
		resp.Body.Close()
	}
}

// BenchmarkGQL2REST measures the cost of translating an inbound GraphQL query
// into one or more REST calls against a mock REST backend, then reassembling
// the GraphQL JSON response.
func BenchmarkGQL2REST(b *testing.B) {
	b.ReportAllocs()

	// Mock REST backend.
	restBackend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		switch r.URL.Path {
		case "/users":
			w.Write([]byte(`[{"id":"1","name":"Alice"},{"id":"2","name":"Bob"}]`))
		default:
			w.Write([]byte(`{"id":"1","name":"Alice"}`))
		}
	}))
	defer restBackend.Close()

	restClient := &http.Client{}

	// GQL-to-REST proxy: accept a GraphQL POST, parse the query,
	// dispatch to the REST backend, re-wrap as GraphQL JSON.
	gql2restHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		r.Body.Close()

		var gqlReq struct {
			Query string `json:"query"`
		}
		json.Unmarshal(body, &gqlReq)

		// Resolve by dispatching to REST.
		restReq, err := http.NewRequestWithContext(r.Context(), http.MethodGet, restBackend.URL+"/users", nil)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		resp, err := restClient.Do(restReq)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		restBody, _ := io.ReadAll(resp.Body)

		// Wrap REST payload as a GraphQL response.
		gqlResp := fmt.Sprintf(`{"data":{"users":%s}}`, string(restBody))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(gqlResp))
	})

	proxy := httptest.NewServer(gql2restHandler)
	defer proxy.Close()

	gqlPayload := `{"query":"{ users { id name } }"}`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequest(http.MethodPost, proxy.URL+"/graphql", strings.NewReader(gqlPayload))
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			b.Fatal(err)
		}
		io.ReadAll(resp.Body)
		resp.Body.Close()
	}
}
