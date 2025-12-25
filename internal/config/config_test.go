package config

import (
	"os"
	"testing"
)

// TestGetEnv tests the GetEnv function.
func TestGetEnv(t *testing.T) {
	// Case 1: Environment variable is set
	t.Run("Environment variable is set", func(t *testing.T) {
		// Set a test environment variable
		testKey := "TEST_ENV_VAR"
		expectedValue := "test_value"
		os.Setenv(testKey, expectedValue)
		// Clear the environment variable after the test
		defer os.Unsetenv(testKey)

		// Call GetEnv
		actualValue := GetEnv(testKey, "default")

		// Verify the result
		if actualValue != expectedValue {
			t.Errorf("Expected %s, Actual %s", expectedValue, actualValue)
		}
	})

	// Case 2: Environment variable is not set
	t.Run("Environment variable is not set", func(t *testing.T) {
		// Non-existent environment variable key
		testKey := "NON_EXISTENT_VAR"
		defaultValue := "default_value"

		// Call GetEnv
		actualValue := GetEnv(testKey, defaultValue)

		// Verify the result
		if actualValue != defaultValue {
			t.Errorf("Expected %s, Actual %s", defaultValue, actualValue)
		}
	})
}

// TestNewSolrClient tests the NewSolrClient function.
func TestNewSolrClient(t *testing.T) {
	// Clear environment variables that may already be set
	os.Unsetenv("SOLR_MCP_SOLR_URL")
	os.Unsetenv("SOLR_BASIC_USER")
	os.Unsetenv("SOLR_BASIC_PASS")

	// Case 1: No environment variables are set
	t.Run("No environment variables are set", func(t *testing.T) {
		_, baseURL, user, pass, _ := NewSolrClient()
		expectedURL := "http://localhost:8983"
		if baseURL != expectedURL {
			t.Errorf("Expected URL %s, Actual %s", expectedURL, baseURL)
		}
		if user != "" {
			t.Errorf("Expected empty username, Actual %s", user)
		}
		if pass != "" {
			t.Errorf("Expected empty password, Actual %s", pass)
		}
	})

	// Case 2: Solr URL is set
	t.Run("Solr URL is set", func(t *testing.T) {
		expectedURL := "http://solr.example.com:8983"
		os.Setenv("SOLR_MCP_SOLR_URL", expectedURL)
		defer os.Unsetenv("SOLR_MCP_SOLR_URL")

		_, baseURL, _, _, _ := NewSolrClient()
		if baseURL != expectedURL {
			t.Errorf("Expected URL %s, Actual %s", expectedURL, baseURL)
		}
	})

	// Case 3: Basic auth credentials are set
	t.Run("Basic auth credentials are set", func(t *testing.T) {
		expectedUser := "testuser"
		expectedPass := "testpass"
		os.Setenv("SOLR_BASIC_USER", expectedUser)
		os.Setenv("SOLR_BASIC_PASS", expectedPass)
		defer os.Unsetenv("SOLR_BASIC_USER")
		defer os.Unsetenv("SOLR_BASIC_PASS")

		_, _, user, pass, _ := NewSolrClient()
		if user != expectedUser {
			t.Errorf("Expected username %s, Actual %s", expectedUser, user)
		}
		if pass != expectedPass {
			t.Errorf("Expected password %s, Actual %s", expectedPass, pass)
		}
	})
}
