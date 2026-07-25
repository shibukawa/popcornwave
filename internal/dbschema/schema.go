// Package dbschema applies version-control-owned schema initialization files.
package dbschema

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Apply executes lexical-order .sql files in one transaction.
func Apply(ctx context.Context, db *sql.DB, directory string) error {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return err
	}
	var paths []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.EqualFold(filepath.Ext(entry.Name()), ".sql") {
			paths = append(paths, filepath.Join(directory, entry.Name()))
		}
	}
	sort.Strings(paths)
	if len(paths) == 0 {
		return fmt.Errorf("no .sql files in %s", directory)
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	for _, path := range paths {
		source, err := os.ReadFile(path)
		if err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("read %s: %w", filepath.Base(path), err)
		}
		if _, err := tx.ExecContext(ctx, string(source)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("apply %s: %w", filepath.Base(path), err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit schema: %w", err)
	}
	return nil
}
