package router

import (
	"errors"
	"reflect"
	"testing"

	"github.com/zurco34/pelican-mc-router/internal/routepolicy"
	"github.com/zurco34/pelican-mc-router/pkg/models"
)

func TestPlannerPlanAppliesPrimaryAndAliases(t *testing.T) {
	t.Parallel()
	planner, err := NewPlanner("mc.example.com")
	if err != nil {
		t.Fatalf("NewPlanner() error = %v", err)
	}

	routes, err := planner.Plan([]models.MinecraftServer{{
		UUID:        "server-uuid",
		Name:        "Generated Default",
		BackendIP:   "192.168.1.10",
		BackendPort: 25565,
	}}, map[string]routepolicy.Policy{
		"server-uuid": {
			ServerUUID:      "server-uuid",
			PrimaryHostname: "Primary.MC.Example.COM",
			Aliases:         []string{"play.mc.example.com"},
		},
	})
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}
	want := []Route{
		{ServerID: "server-uuid", Hostname: "primary.mc.example.com", Backend: Backend{Host: "192.168.1.10", Port: 25565}},
		{ServerID: "server-uuid", Hostname: "play.mc.example.com", Backend: Backend{Host: "192.168.1.10", Port: 25565}},
	}
	if !reflect.DeepEqual(routes, want) {
		t.Fatalf("Plan() = %#v, want %#v", routes, want)
	}
}

func TestPlannerPlanRejectsInvalidCompleteSets(t *testing.T) {
	t.Parallel()
	planner, err := NewPlanner("mc.example.com")
	if err != nil {
		t.Fatalf("NewPlanner() error = %v", err)
	}

	valid := models.MinecraftServer{UUID: "one", Name: "one", BackendIP: "192.168.1.10", BackendPort: 25565}
	tests := []struct {
		name     string
		servers  []models.MinecraftServer
		policies map[string]routepolicy.Policy
	}{
		{name: "missing UUID", servers: []models.MinecraftServer{{Name: "one", BackendIP: "192.168.1.10", BackendPort: 25565}}},
		{name: "duplicate normalized defaults", servers: []models.MinecraftServer{valid, {UUID: "two", Name: "ONE", BackendIP: "192.168.1.11", BackendPort: 25565}}},
		{name: "invalid policy hostname", servers: []models.MinecraftServer{valid}, policies: map[string]routepolicy.Policy{"one": {ServerUUID: "one", PrimaryHostname: "bad_name"}}},
		{name: "duplicate alias", servers: []models.MinecraftServer{valid}, policies: map[string]routepolicy.Policy{"one": {ServerUUID: "one", Aliases: []string{"one.mc.example.com"}}}},
		{name: "excluded policy has routes", servers: []models.MinecraftServer{valid}, policies: map[string]routepolicy.Policy{"one": {ServerUUID: "one", Excluded: true, Aliases: []string{"play.mc.example.com"}}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := planner.Plan(test.servers, test.policies)
			if !errors.Is(err, ErrInvalidRoutePlan) {
				t.Fatalf("Plan() error = %v, want ErrInvalidRoutePlan", err)
			}
		})
	}
}

func TestPlannerPlanSkipsSuspendedAndExcludedServers(t *testing.T) {
	t.Parallel()
	planner, err := NewPlanner("mc.example.com")
	if err != nil {
		t.Fatalf("NewPlanner() error = %v", err)
	}

	routes, err := planner.Plan([]models.MinecraftServer{
		{UUID: "suspended", Name: "suspended", BackendIP: "192.168.1.10", BackendPort: 25565, Suspended: true},
		{UUID: "excluded", Name: "excluded", BackendIP: "192.168.1.11", BackendPort: 25565},
	}, map[string]routepolicy.Policy{"excluded": {ServerUUID: "excluded", Excluded: true}})
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}
	if len(routes) != 0 {
		t.Fatalf("Plan() routes = %#v, want none", routes)
	}
}
