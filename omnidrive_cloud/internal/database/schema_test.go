package database

import (
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestSplitSQLStatements(t *testing.T) {
	statements := splitSQLStatements(`
CREATE TABLE foo (
    id TEXT PRIMARY KEY,
    template TEXT NOT NULL DEFAULT '{"code":"##code##"}'
);
ALTER TABLE foo ADD COLUMN IF NOT EXISTS notes TEXT;
`)

	if len(statements) != 2 {
		t.Fatalf("expected 2 statements, got %d", len(statements))
	}
	if statements[0] == "" || statements[1] == "" {
		t.Fatal("split statements should not contain empty items")
	}
}

func TestShouldIgnoreSchemaError(t *testing.T) {
	err := &pgconn.PgError{
		Code:    "42501",
		Message: "must be owner of table phone_verification_codes",
	}

	if !shouldIgnoreSchemaError("ALTER TABLE phone_verification_codes ADD COLUMN IF NOT EXISTS verification_code_hash TEXT", err) {
		t.Fatal("expected ALTER TABLE ownership error to be ignored")
	}
	if shouldIgnoreSchemaError("CREATE TABLE users (id TEXT PRIMARY KEY)", err) {
		t.Fatal("did not expect CREATE TABLE permission error to be ignored")
	}
}

func TestBootstrapSQLIncludesPromptOptimizeModel(t *testing.T) {
	if !strings.Contains(bootstrapSQL, "prompt_optimize_model") {
		t.Fatal("expected bootstrapSQL to define prompt_optimize_model")
	}
	if !strings.Contains(bootstrapSQL, "claude-opus-4-6-thinking") {
		t.Fatal("expected bootstrapSQL to backfill old claude prompt optimize model values")
	}
}
