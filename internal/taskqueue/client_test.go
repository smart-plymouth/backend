package taskqueue

import (
	"testing"
)

func TestNewClientFallback(t *testing.T) {
	// Test that an invalid Redis URL results in a fallback client (doesn't panic)
	client := NewClient("invalid-url")
	if client == nil {
		t.Fatal("NewClient returned nil for invalid URL")
	}
	defer client.Close()
}

func TestNewClientValidURL(t *testing.T) {
	// Test that a valid Redis URL parses correctly (doesn't panic)
	client := NewClient("redis://localhost:6379/0")
	if client == nil {
		t.Fatal("NewClient returned nil for valid URL")
	}
	defer client.Close()
}

func TestNewClientWithDB(t *testing.T) {
	// Test Redis URL with database number
	client := NewClient("redis://localhost:6379/5")
	if client == nil {
		t.Fatal("NewClient returned nil")
	}
	defer client.Close()
}
