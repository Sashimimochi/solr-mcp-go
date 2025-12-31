package server

import (
	"testing"
)

// TestNewServerState tests the NewServerState function.
func TestNewServerState(t *testing.T) {
	// Goal: When environment variables are set, ensure NewServerState initializes State correctly.
	t.Run("With environment variables", func(t *testing.T) {
		t.Setenv("SOLR_MCP_DEFAULT_COLLECTION", "test_collection")

		state := NewServerState()

		if state.DefaultCollection != "test_collection" {
			t.Errorf("Expected DefaultCollection to be 'test_collection', got '%s'", state.DefaultCollection)
		}
	})

	// Goal: When environment variables are not set, ensure State initializes with defaults.
	t.Run("Without environment variables (defaults)", func(t *testing.T) {
		// Note: t.Setenv() applies only within a subtest; no env vars here.
		state := NewServerState()

		if state.DefaultCollection != "gettingstarted" {
			t.Errorf("Expected DefaultCollection to be 'gettingstarted', got '%s'", state.DefaultCollection)
		}
	})
}
