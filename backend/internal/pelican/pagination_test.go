package pelican

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

type testResource struct {
	ID int `json:"id"`
}

func TestListAll(t *testing.T) {
	t.Run("retrieves every page", func(t *testing.T) {
		requestedPages := make([]string, 0, 2)

		server := httptest.NewServer(http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				page := r.URL.Query().Get("page")
				requestedPages = append(requestedPages, page)

				response := ListResponse[testResource]{
					Object: "list",
					Meta: ResponseMeta{
						Pagination: Pagination{
							Total:       2,
							Count:       1,
							PerPage:     1,
							CurrentPage: 1,
							TotalPages:  2,
						},
					},
				}

				switch page {
				case "1":
					response.Data = []testResource{{ID: 1}}

				case "2":
					response.Data = []testResource{{ID: 2}}

				default:
					http.Error(
						w,
						"unexpected page",
						http.StatusBadRequest,
					)
					return
				}

				w.Header().Set("Content-Type", "application/json")

				if err := json.NewEncoder(w).Encode(response); err != nil {
					t.Errorf("encode response: %v", err)
					return
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

		items, err := listAll[testResource](
			context.Background(),
			client,
			"/resources",
		)
		if err != nil {
			t.Fatalf("listAll() error = %v", err)
		}

		if len(items) != 2 {
			t.Fatalf("expected 2 items, got %d", len(items))
		}

		if items[0].ID != 1 || items[1].ID != 2 {
			t.Fatalf("unexpected items: %+v", items)
		}

		if len(requestedPages) != 2 {
			t.Fatalf(
				"expected 2 requests, got %d",
				len(requestedPages),
			)
		}

		if requestedPages[0] != "1" ||
			requestedPages[1] != "2" {
			t.Fatalf(
				"unexpected requested pages: %v",
				requestedPages,
			)
		}
	})

	t.Run("treats missing pagination as one page", func(t *testing.T) {
		requestCount := 0

		server := httptest.NewServer(http.HandlerFunc(
			func(w http.ResponseWriter, _ *http.Request) {
				requestCount++

				w.Header().Set("Content-Type", "application/json")

				response := ListResponse[testResource]{
					Object: "list",
					Data: []testResource{
						{ID: 1},
					},
				}

				if err := json.NewEncoder(w).Encode(response); err != nil {
					t.Errorf("encode response: %v", err)
					return
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

		items, err := listAll[testResource](
			context.Background(),
			client,
			"/resources",
		)
		if err != nil {
			t.Fatalf("listAll() error = %v", err)
		}

		if len(items) != 1 {
			t.Fatalf("expected 1 item, got %d", len(items))
		}

		if requestCount != 1 {
			t.Fatalf(
				"expected 1 request, got %d",
				requestCount,
			)
		}
	})
}
