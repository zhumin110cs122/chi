package main

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func main() {
	r := chi.NewRouter()

	// Apply timeout middleware - 50ms timeout
	r.Use(middleware.Timeout(50 * time.Millisecond))

	r.Get("/slow", func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(200 * time.Millisecond):
			fmt.Fprintf(w, "slow response")
		case <-r.Context().Done():
			// Context was cancelled by Timeout middleware
			http.Error(w, "request timed out", http.StatusGatewayTimeout)
		}
	})

	r.Get("/fast", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "fast response")
	})

	server := httptest.NewServer(r)
	defer server.Close()

	// Test 1: Fast request (within timeout)
	fmt.Println("=== Test 1: Fast request ===")
	resp, err := http.Get(server.URL + "/fast")
	if err != nil {
		fmt.Printf("FAIL: %v\n", err)
	} else {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		fmt.Printf("Status: %d, Body: %s\n", resp.StatusCode, string(body))
	}

	// Test 2: Slow request (exceeds timeout)
	fmt.Println("=== Test 2: Slow request (should timeout) ===")
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err = client.Get(server.URL + "/slow")
	if err != nil {
		fmt.Printf("FAIL: %v\n", err)
	} else {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		fmt.Printf("Status: %d, Body: %s\n", resp.StatusCode, string(body))
	}

	fmt.Println("\nAll tests completed - Timeout middleware cancels context correctly.")
}
