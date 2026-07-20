package pelican

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestListNodeAllocations(t *testing.T) {
	requestedPages := make([]string, 0, 2)

	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/api/application/nodes/1/allocations" {
				t.Errorf(
					"unexpected request path: %q",
					r.URL.Path,
				)
				http.Error(
					w,
					"unexpected request path",
					http.StatusNotFound,
				)
				return
			}

			page := r.URL.Query().Get("page")
			requestedPages = append(requestedPages, page)

			w.Header().Set("Content-Type", "application/json")

			switch page {
			case "1":
				writeAllocationListResponse(
					t,
					w,
					ListResponse[AllocationResource]{
						Object: "list",
						Data: []AllocationResource{
							{
								Object: "allocation",
								Attributes: AllocationAttributes{
									ID:       1,
									IP:       "0.0.0.0",
									Alias:    stringPointer("Minecraft"),
									Port:     25566,
									Assigned: true,
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
					},
				)

			case "2":
				writeAllocationListResponse(
					t,
					w,
					ListResponse[AllocationResource]{
						Object: "list",
						Data: []AllocationResource{
							{
								Object: "allocation",
								Attributes: AllocationAttributes{
									ID:       2,
									IP:       "192.168.1.10",
									Alias:    nil,
									Port:     25567,
									Notes:    stringPointer("modded server"),
									Assigned: false,
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
					},
				)

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

	allocations, err := client.ListNodeAllocations(
		context.Background(),
		1,
	)
	if err != nil {
		t.Fatalf("ListNodeAllocations() error = %v", err)
	}

	if len(allocations) != 2 {
		t.Fatalf(
			"expected 2 allocations, got %d",
			len(allocations),
		)
	}

	first := allocations[0].Attributes

	if first.ID != 1 {
		t.Errorf("unexpected first allocation ID: %d", first.ID)
	}

	if first.IP != "0.0.0.0" {
		t.Errorf("unexpected first allocation IP: %q", first.IP)
	}

	if first.Port != 25566 {
		t.Errorf("unexpected first allocation port: %d", first.Port)
	}

	if first.Alias == nil || *first.Alias != "Minecraft" {
		t.Errorf("unexpected first allocation alias: %v", first.Alias)
	}

	if !first.Assigned {
		t.Error("expected first allocation to be assigned")
	}

	second := allocations[1].Attributes

	if second.ID != 2 {
		t.Errorf("unexpected second allocation ID: %d", second.ID)
	}

	if second.Alias != nil {
		t.Errorf("expected second allocation alias to be nil")
	}

	if second.Notes == nil || *second.Notes != "modded server" {
		t.Errorf(
			"unexpected second allocation notes: %v",
			second.Notes,
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

func TestListNodeAllocationsRejectsInvalidNodeID(t *testing.T) {
	client := &Client{}

	tests := []struct {
		name   string
		nodeID int
	}{
		{
			name:   "zero",
			nodeID: 0,
		},
		{
			name:   "negative",
			nodeID: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := client.ListNodeAllocations(
				context.Background(),
				tt.nodeID,
			)

			if err == nil {
				t.Fatal(
					"ListNodeAllocations() error = nil, want error",
				)
			}
		})
	}
}

func writeAllocationListResponse(
	t *testing.T,
	w http.ResponseWriter,
	response ListResponse[AllocationResource],
) {
	t.Helper()

	if err := json.NewEncoder(w).Encode(response); err != nil {
		t.Errorf("encode response: %v", err)
	}
}

func stringPointer(value string) *string {
	return &value
}
