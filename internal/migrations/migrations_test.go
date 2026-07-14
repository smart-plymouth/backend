package migrations

import (
	"sort"
	"testing"

	"gorm.io/gorm"
)

func TestRegister(t *testing.T) {
	origRegistry := registry
	defer func() { registry = origRegistry }()

	registry = nil
	Register(Migration{
		Version: "test_001",
		Up:      func(tx *gorm.DB) error { return nil },
		Down:    func(tx *gorm.DB) error { return nil },
	})

	if len(registry) != 1 {
		t.Fatalf("expected 1 migration in registry, got %d", len(registry))
	}
	if registry[0].Version != "test_001" {
		t.Errorf("unexpected version: %s", registry[0].Version)
	}
}

func TestRegisterMultiple(t *testing.T) {
	origRegistry := registry
	defer func() { registry = origRegistry }()

	registry = nil
	Register(Migration{Version: "001", Up: func(tx *gorm.DB) error { return nil }, Down: func(tx *gorm.DB) error { return nil }})
	Register(Migration{Version: "002", Up: func(tx *gorm.DB) error { return nil }, Down: func(tx *gorm.DB) error { return nil }})
	Register(Migration{Version: "003", Up: func(tx *gorm.DB) error { return nil }, Down: func(tx *gorm.DB) error { return nil }})

	if len(registry) != 3 {
		t.Fatalf("expected 3 migrations, got %d", len(registry))
	}
}

func TestMigrationRecordTableName(t *testing.T) {
	r := MigrationRecord{}
	if r.TableName() != "schema_migrations" {
		t.Errorf("expected 'schema_migrations', got %q", r.TableName())
	}
}

func TestRegistryOrdering(t *testing.T) {
	origRegistry := registry
	defer func() { registry = origRegistry }()

	registry = nil
	// Register in reverse order
	Register(Migration{Version: "003", Up: func(tx *gorm.DB) error { return nil }, Down: func(tx *gorm.DB) error { return nil }})
	Register(Migration{Version: "001", Up: func(tx *gorm.DB) error { return nil }, Down: func(tx *gorm.DB) error { return nil }})
	Register(Migration{Version: "002", Up: func(tx *gorm.DB) error { return nil }, Down: func(tx *gorm.DB) error { return nil }})

	// Simulate what Up() does: sort by version
	sort.Slice(registry, func(i, j int) bool {
		return registry[i].Version < registry[j].Version
	})

	if registry[0].Version != "001" {
		t.Errorf("expected first migration to be 001, got %s", registry[0].Version)
	}
	if registry[1].Version != "002" {
		t.Errorf("expected second migration to be 002, got %s", registry[1].Version)
	}
	if registry[2].Version != "003" {
		t.Errorf("expected third migration to be 003, got %s", registry[2].Version)
	}
}

func TestRegistryHasAllMigrations(t *testing.T) {
	// The real init() functions register all migrations.
	// Verify we have the expected number (13 migration files).
	expectedVersions := []string{
		"001", "002", "003", "004", "005", "006",
		"007", "008", "009", "010", "011", "012", "013",
	}

	versions := make(map[string]bool)
	for _, m := range registry {
		versions[m.Version] = true
	}

	for _, v := range expectedVersions {
		if !versions[v] {
			t.Errorf("migration version %s not found in registry", v)
		}
	}

	if len(registry) < len(expectedVersions) {
		t.Errorf("expected at least %d migrations, got %d", len(expectedVersions), len(registry))
	}
}

func TestMigrationUpDownNotNil(t *testing.T) {
	for _, m := range registry {
		if m.Up == nil {
			t.Errorf("migration %s has nil Up function", m.Version)
		}
		if m.Down == nil {
			t.Errorf("migration %s has nil Down function", m.Version)
		}
	}
}
