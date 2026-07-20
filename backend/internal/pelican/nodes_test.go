package pelican

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

func TestListNodes(t *testing.T) {
	t.Run("lists and maps nodes", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/application/nodes" {
					t.Fatalf(
						"request path = %q, want %q",
						r.URL.Path,
						"/api/application/nodes",
					)
				}

				writeNodeListResponse(t, w, []NodeResource{
					{
						Object: "node",
						Attributes: NodeAttributes{
							ID:                 1,
							UUID:               "node-uuid",
							Public:             true,
							Name:               "Primary node",
							FQDN:               "wings.example.com",
							Scheme:             "https",
							BehindProxy:        true,
							MaintenanceMode:    false,
							Memory:             32768,
							MemoryOverallocate: 0,
							Disk:               100000,
							DiskOverallocate:   0,
							UploadSize:         100,
							DaemonListen:       8080,
							DaemonSFTP:         2022,
							DaemonBase:         "/var/lib/pelican/volumes",
						},
					},
				}, 1, 1)
			},
		))
		defer server.Close()

		client, err := NewClient(Config{
			BaseURL: server.URL + "/api/application",
			APIKey:  "test-api-key",
		})
		if err != nil {
			t.Fatalf("NewClient() error = %v", err)
		}

		nodes, err := client.ListNodes(context.Background())
		if err != nil {
			t.Fatalf("ListNodes() error = %v", err)
		}

		if len(nodes) != 1 {
			t.Fatalf("len(nodes) = %d, want 1", len(nodes))
		}

		node := nodes[0]

		if node.Object != "node" {
			t.Errorf("Object = %q, want %q", node.Object, "node")
		}

		if node.Attributes.ID != 1 {
			t.Errorf("ID = %d, want 1", node.Attributes.ID)
		}

		if node.Attributes.Name != "Primary node" {
			t.Errorf(
				"Name = %q, want %q",
				node.Attributes.Name,
				"Primary node",
			)
		}

		if node.Attributes.FQDN != "wings.example.com" {
			t.Errorf(
				"FQDN = %q, want %q",
				node.Attributes.FQDN,
				"wings.example.com",
			)
		}

		if node.Attributes.DaemonListen != 8080 {
			t.Errorf(
				"DaemonListen = %d, want 8080",
				node.Attributes.DaemonListen,
			)
		}

		if !node.Attributes.BehindProxy {
			t.Error("BehindProxy = false, want true")
		}
	})

	t.Run("follows pagination", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				page, err := strconv.Atoi(r.URL.Query().Get("page"))
				if err != nil {
					t.Fatalf("invalid page query: %v", err)
				}

				switch page {
				case 1:
					writeNodeListResponse(t, w, []NodeResource{
						{
							Object: "node",
							Attributes: NodeAttributes{
								ID:   1,
								Name: "Node one",
								FQDN: "node-one.example.com",
							},
						},
					}, 1, 2)
				case 2:
					writeNodeListResponse(t, w, []NodeResource{
						{
							Object: "node",
							Attributes: NodeAttributes{
								ID:   2,
								Name: "Node two",
								FQDN: "node-two.example.com",
							},
						},
					}, 2, 2)
				default:
					t.Fatalf("unexpected page %d", page)
				}
			},
		))
		defer server.Close()

		client, err := NewClient(Config{
			BaseURL: server.URL + "/api/application",
			APIKey:  "test-api-key",
		})
		if err != nil {
			t.Fatalf("NewClient() error = %v", err)
		}

		nodes, err := client.ListNodes(context.Background())
		if err != nil {
			t.Fatalf("ListNodes() error = %v", err)
		}

		if len(nodes) != 2 {
			t.Fatalf("len(nodes) = %d, want 2", len(nodes))
		}

		if nodes[0].Attributes.ID != 1 {
			t.Errorf(
				"nodes[0].ID = %d, want 1",
				nodes[0].Attributes.ID,
			)
		}

		if nodes[1].Attributes.ID != 2 {
			t.Errorf(
				"nodes[1].ID = %d, want 2",
				nodes[1].Attributes.ID,
			)
		}
	})
}

func writeNodeListResponse(
	t *testing.T,
	w http.ResponseWriter,
	nodes []NodeResource,
	currentPage int,
	totalPages int,
) {
	t.Helper()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	_, err := fmt.Fprintf(
		w,
		`{
			"object": "list",
			"data": %s,
			"meta": {
				"pagination": {
					"total": %d,
					"count": %d,
					"per_page": 50,
					"current_page": %d,
					"total_pages": %d,
					"links": {}
				}
			}
		}`,
		mustMarshalJSON(t, nodes),
		len(nodes)*totalPages,
		len(nodes),
		currentPage,
		totalPages,
	)
	if err != nil {
		t.Fatalf("write response: %v", err)
	}
}
func mustMarshalJSON(t *testing.T, value any) string {
	t.Helper()

	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal JSON: %v", err)
	}

	return string(data)
}
