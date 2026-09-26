package handlers

import (
    "database/sql"
    "path/filepath"
    "testing"

    "social-network/pkg/db/sqlite"
)

// newTestDB spins up a fresh, throwaway SQLite database with every
// migration applied, backing it with a temp file (an in-memory :memory:
// DB doesn't work here since golang-migrate opens its own separate
// connection to apply migrations, which would see an empty database).
// The caller gets a real, fully-migrated schema to exercise handlers
// against - the same schema the actual server runs, not a hand-rolled
// subset that could drift from the real migrations.
//
// The demo data from migration 000022 is removed again (the same way its
// down migration does), so tests start from an empty database and can
// count what they create. newSeededTestDB keeps it.
func newTestDB(t *testing.T) *sql.DB {
    t.Helper()
    db := newSeededTestDB(t)
    if _, err := db.Exec(`DELETE FROM users WHERE email LIKE '%@demo.test'`); err != nil {
        t.Fatalf("failed to remove demo data: %v", err)
    }
    return db
}

// newSeededTestDB is newTestDB with the demo data still in it.
func newSeededTestDB(t *testing.T) *sql.DB {
    t.Helper()

    dbPath := filepath.Join(t.TempDir(), "test.db")
    migrationsPath, err := filepath.Abs("../db/migrations/sqlite")
    if err != nil {
        t.Fatalf("failed to resolve migrations path: %v", err)
    }

    db, err := sqlite.NewDB(sqlite.Config{
        DBPath:            dbPath,
        MigrationsPath:    migrationsPath,
        EnableForeignKeys: true,
    })
    if err != nil {
        t.Fatalf("failed to open test database: %v", err)
    }
    t.Cleanup(func() { db.Close() })

    if err := db.ApplyMigrations(migrationsPath); err != nil {
        t.Fatalf("failed to apply migrations: %v", err)
    }

    return db.DB
}
