package sqlite

import (
    "context"
    "database/sql"
    "fmt"
    "log"
    "os"
    "path/filepath"

    "github.com/golang-migrate/migrate/v4"
    "github.com/golang-migrate/migrate/v4/database/sqlite3"
    _ "github.com/golang-migrate/migrate/v4/source/file"
    _ "github.com/mattn/go-sqlite3"
)

type DB struct {
    *sql.DB
}

type Config struct {
    DBPath          string
    MigrationsPath  string
    EnableForeignKeys bool
}

func NewDB(config Config) (*DB, error) {
    // Ensure the directory exists
    dir := filepath.Dir(config.DBPath)
    if err := os.MkdirAll(dir, 0755); err != nil {
        return nil, fmt.Errorf("failed to create database directory: %v", err)
    }

    // Build DSN
    dsn := config.DBPath
    if config.EnableForeignKeys {
        dsn += "?_foreign_keys=on"
    }

    log.Printf("Opening database at: %s", dsn)

    db, err := sql.Open("sqlite3", dsn)
    if err != nil {
        return nil, fmt.Errorf("failed to open database: %v", err)
    }

    ctx := context.Background()
    if err := db.PingContext(ctx); err != nil {
        return nil, fmt.Errorf("failed to ping database: %v", err)
    }

    db.SetMaxOpenConns(25)
    db.SetMaxIdleConns(25)
    db.SetConnMaxLifetime(0)

    log.Println("Database connection established successfully")
    return &DB{db}, nil
}

// ApplyMigrations applies all pending migrations
func (db *DB) ApplyMigrations(migrationsPath string) error {
    // Check if migrations directory exists
    if _, err := os.Stat(migrationsPath); os.IsNotExist(err) {
        return fmt.Errorf("migrations directory does not exist: %s", migrationsPath)
    }

    // Convert to absolute path if relative
    absPath, err := filepath.Abs(migrationsPath)
    if err != nil {
        return fmt.Errorf("failed to get absolute path: %v", err)
    }

    log.Printf("Applying migrations from: %s", absPath)

    // Create migration driver
    driver, err := sqlite3.WithInstance(db.DB, &sqlite3.Config{
        DatabaseName: "main",
        NoTxWrap:     false,
    })
    if err != nil {
        return fmt.Errorf("failed to create migration driver: %v", err)
    }

    // Create migration instance with file:// prefix
    m, err := migrate.NewWithDatabaseInstance(
        "file://"+absPath,
        "sqlite3",
        driver,
    )
    if err != nil {
        return fmt.Errorf("failed to create migration instance: %v", err)
    }

    // Apply migrations
    if err := m.Up(); err != nil && err != migrate.ErrNoChange {
        return fmt.Errorf("failed to apply migrations: %v", err)
    }

    if err == migrate.ErrNoChange {
        log.Println("No new migrations to apply")
    } else {
        log.Println("Migrations applied successfully")
    }

    return nil
}

// Close closes the database connection
func (db *DB) Close() error {
    log.Println("Closing database connection...")
    return db.DB.Close()
}

// Ping verifies a connection to the database is still alive
func (db *DB) Ping() error {
    ctx := context.Background()
    return db.DB.PingContext(ctx)
}

// HealthCheck performs a health check on the database
func (db *DB) HealthCheck() error {
    ctx := context.Background()
    if err := db.DB.PingContext(ctx); err != nil {
        return fmt.Errorf("database health check failed: %v", err)
    }

    _, err := db.DB.ExecContext(ctx, "SELECT 1")
    if err != nil {
        return fmt.Errorf("database query failed: %v", err)
    }

    return nil
}