package client_test

import (
	"testing"

	"github.com/edwinzhancn/lumen-sdk/pkg/discovery"
	pb "github.com/edwinzhancn/lumen-sdk/proto"
)

func TestNodeInfoIsActiveRequiresReadyAndCompatible(t *testing.T) {
	tests := []struct {
		name          string
		availability  discovery.NodeAvailability
		compatibility discovery.CompatibilityState
		want          bool
	}{
		{"ready compatible", discovery.NodeAvailabilityReady, discovery.CompatibilityCompatible, true},
		{"ready pending", discovery.NodeAvailabilityReady, discovery.CompatibilityPending, false},
		{"ready incompatible", discovery.NodeAvailabilityReady, discovery.CompatibilityIncompatible, false},
		{"connecting compatible", discovery.NodeAvailabilityConnecting, discovery.CompatibilityCompatible, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := &discovery.NodeInfo{Availability: tt.availability, Compatibility: tt.compatibility}
			if got := node.IsActive(); got != tt.want {
				t.Fatalf("IsActive() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNodeInfoTaskQueriesUseCapabilities(t *testing.T) {
	node := &discovery.NodeInfo{Capabilities: []*pb.Capability{
		{ServiceName: "vision", Tasks: []*pb.IOTask{{Name: "ocr"}, {Name: "embed"}}},
		{ServiceName: "semantic", Tasks: []*pb.IOTask{{Name: "embed"}}},
	}}

	if !node.SupportsTask("ocr") || !node.SupportsTask("embed") {
		t.Fatal("tasks declared by capabilities must be supported")
	}
	if node.SupportsTask("missing") || node.SupportsTask("") {
		t.Fatal("unknown or empty tasks must not be supported")
	}
	if !node.SupportsServiceTask("vision", "ocr") || node.SupportsServiceTask("semantic", "ocr") {
		t.Fatal("service-qualified task query returned the wrong result")
	}
	services := node.MatchingServices("embed")
	if len(services) != 2 || services[0] != "vision" || services[1] != "semantic" {
		t.Fatalf("MatchingServices(embed) = %v, want [vision semantic]", services)
	}
}

func TestOperationalStateConstantsAreDistinct(t *testing.T) {
	availability := []discovery.NodeAvailability{
		discovery.NodeAvailabilityDiscovered,
		discovery.NodeAvailabilityConnecting,
		discovery.NodeAvailabilityReady,
		discovery.NodeAvailabilityUnavailable,
	}
	compatibility := []discovery.CompatibilityState{
		discovery.CompatibilityPending,
		discovery.CompatibilityCompatible,
		discovery.CompatibilityIncompatible,
	}
	assertDistinct := func(values []string) {
		t.Helper()
		seen := make(map[string]bool, len(values))
		for _, value := range values {
			if value == "" || seen[value] {
				t.Fatalf("state values must be non-empty and distinct: %v", values)
			}
			seen[value] = true
		}
	}
	availabilityValues := make([]string, 0, len(availability))
	for _, value := range availability {
		availabilityValues = append(availabilityValues, string(value))
	}
	compatibilityValues := make([]string, 0, len(compatibility))
	for _, value := range compatibility {
		compatibilityValues = append(compatibilityValues, string(value))
	}
	assertDistinct(availabilityValues)
	assertDistinct(compatibilityValues)
}

func TestModelInfoStruct(t *testing.T) {
	model := discovery.ModelInfo{ID: "model-123", Name: "Test Model", Version: "1.0.0", Runtime: "cuda"}
	if model.ID != "model-123" || model.Name != "Test Model" || model.Version != "1.0.0" || model.Runtime != "cuda" {
		t.Fatalf("unexpected ModelInfo: %+v", model)
	}
}
