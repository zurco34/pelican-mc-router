package pelican

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestListServers(t *testing.T) {
	requestedPages := make([]string, 0, 2)

	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			page := r.URL.Query().Get("page")
			requestedPages = append(requestedPages, page)

			w.Header().Set("Content-Type", "application/json")

			switch page {
			case "1":
				writeServerListResponse(t, w, ListResponse[ServerResource]{
					Object: "list",
					Data: []ServerResource{
						{
							Object: "server",
							Attributes: ServerAttributes{
								ID:   1,
								UUID: "server-one",
								Name: "Server One",
							},
						},
					},
					Meta: ResponseMeta{
						Pagination: Pagination{
							Total:       2,
							Count:       1,
							PerPage:     1,
							CurrentPage: 1,
							TotalPages:  2,
						},
					},
				})

			case "2":
				writeServerListResponse(t, w, ListResponse[ServerResource]{
					Object: "list",
					Data: []ServerResource{
						{
							Object: "server",
							Attributes: ServerAttributes{
								ID:   2,
								UUID: "server-two",
								Name: "Server Two",
							},
						},
					},
					Meta: ResponseMeta{
						Pagination: Pagination{
							Total:       2,
							Count:       1,
							PerPage:     1,
							CurrentPage: 2,
							TotalPages:  2,
						},
					},
				})

			default:
				t.Errorf("unexpected page: %q", page)
				http.Error(
					w,
					"unexpected page",
					http.StatusBadRequest,
				)
			}
		},
	))
	defer server.Close()

	client, err := NewClient(Config{
		BaseURL: server.URL + "/api/application",
		APIKey:  "test-token",
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	servers, err := client.ListServers(context.Background())
	if err != nil {
		t.Fatalf("ListServers() error = %v", err)
	}

	if len(servers) != 2 {
		t.Fatalf("expected 2 servers, got %d", len(servers))
	}

	if servers[0].Attributes.UUID != "server-one" {
		t.Errorf(
			"unexpected first server UUID: %q",
			servers[0].Attributes.UUID,
		)
	}

	if servers[1].Attributes.UUID != "server-two" {
		t.Errorf(
			"unexpected second server UUID: %q",
			servers[1].Attributes.UUID,
		)
	}

	if len(requestedPages) != 2 {
		t.Fatalf(
			"expected 2 page requests, got %d",
			len(requestedPages),
		)
	}

	if requestedPages[0] != "1" || requestedPages[1] != "2" {
		t.Fatalf(
			"unexpected requested pages: %v",
			requestedPages,
		)
	}
}

func writeServerListResponse(
	t *testing.T,
	w http.ResponseWriter,
	response ListResponse[ServerResource],
) {
	t.Helper()

	if err := json.NewEncoder(w).Encode(response); err != nil {
		t.Fatalf("encode response: %v", err)
	}
}
func TestListServersIgnoresStaleCurrentPage(t *testing.T) {
	requestCount := 0

	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			requestCount++

			page := r.URL.Query().Get("page")

			response := ListResponse[ServerResource]{
				Object: "list",
				Meta: ResponseMeta{
					Pagination: Pagination{
						CurrentPage: 1,
						TotalPages:  2,
					},
				},
			}

			if page == "1" {
				response.Data = []ServerResource{
					{
						Attributes: ServerAttributes{
							UUID: "server-one",
						},
					},
				}
			}

			if page == "2" {
				response.Data = []ServerResource{
					{
						Attributes: ServerAttributes{
							UUID: "server-two",
						},
					},
				}
			}

			w.Header().Set("Content-Type", "application/json")

			if err := json.NewEncoder(w).Encode(response); err != nil {
				t.Fatalf("encode response: %v", err)
			}
		},
	))
	defer server.Close()

	client, err := NewClient(Config{
		BaseURL: server.URL + "/api/application",
		APIKey:  "test-token",
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	servers, err := client.ListServers(context.Background())
	if err != nil {
		t.Fatalf("ListServers() error = %v", err)
	}

	if len(servers) != 2 {
		t.Fatalf("expected 2 servers, got %d", len(servers))
	}

	if requestCount != 2 {
		t.Fatalf("expected 2 requests, got %d", requestCount)
	}
}
