package database

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestShouldRunSeedMigrations(t *testing.T) {
	for _, tc := range []struct {
		name  string
		value string
		want  bool
	}{
		{name: "unset", value: "", want: false},
		{name: "disabled", value: "0", want: false},
		{name: "true", value: "true", want: true},
		{name: "one", value: "1", want: true},
		{name: "case and spaces", value: " YES ", want: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("VOLA_RUN_SEED_MIGRATIONS", tc.value)
			if got := shouldRunSeedMigrations(); got != tc.want {
				t.Fatalf("shouldRunSeedMigrations() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestListMigrationFilesSkipsSeedMigrationsByDefault(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{
		"001_initial_schema.sql",
		"002_seed_data.sql",
		"005_realistic_seed_data.sql",
		"010_widen_oauth_client_id.sql",
		"027_rename_seed_neudrive_to_vola.sql",
		"notes.md",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("-- test\n"), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "099_directory.sql"), 0o755); err != nil {
		t.Fatalf("create migration-like directory: %v", err)
	}

	got, err := listMigrationFiles(dir, false)
	if err != nil {
		t.Fatalf("listMigrationFiles: %v", err)
	}
	want := []string{"001_initial_schema.sql", "010_widen_oauth_client_id.sql"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("files = %#v, want %#v", got, want)
	}
}

func TestListMigrationFilesCanIncludeSeedMigrations(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{
		"002_seed_data.sql",
		"001_initial_schema.sql",
		"005_realistic_seed_data.sql",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("-- test\n"), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	got, err := listMigrationFiles(dir, true)
	if err != nil {
		t.Fatalf("listMigrationFiles: %v", err)
	}
	want := []string{"001_initial_schema.sql", "002_seed_data.sql", "005_realistic_seed_data.sql"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("files = %#v, want %#v", got, want)
	}
}
