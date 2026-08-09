package main

import (
	"encoding/json"
	"net/http"
)

func main() {
	http.HandleFunc("/api/application/", handler)
	if err := http.ListenAndServe(":8081", nil); err != nil {
		panic(err)
	}
}

func handler(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Authorization") != "Bearer lifecycle-pelican-key" {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	var data any
	switch r.URL.Path {
	case "/api/application/nodes":
		data = []any{resource("node", map[string]any{"id": 1, "uuid": "node-uuid", "name": "node", "fqdn": "127.0.0.1"})}
	case "/api/application/servers":
		data = []any{resource("server", map[string]any{"id": 1, "uuid": "server-uuid", "identifier": "lifecycle", "name": "lifecycle", "node": 1, "allocation": 1, "egg": 1})}
	case "/api/application/eggs":
		data = []any{resource("egg", map[string]any{"id": 1, "uuid": "egg-uuid", "name": "Minecraft", "tags": []string{"minecraft"}})}
	case "/api/application/nodes/1/allocations":
		data = []any{resource("allocation", map[string]any{"id": 1, "ip": "127.0.0.1", "port": 25565})}
	default:
		w.WriteHeader(http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"object": "list", "data": data, "meta": map[string]any{"pagination": map[string]any{"total": 1, "count": 1, "per_page": 1, "current_page": 1, "total_pages": 1}}})
}

func resource(object string, attributes map[string]any) map[string]any {
	return map[string]any{"object": object, "attributes": attributes}
}
