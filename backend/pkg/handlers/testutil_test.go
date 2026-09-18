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
func newTestDB(t *testing.T) *sql.DB {
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
