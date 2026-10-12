package linter

import (
	"testing"

	"github.com/sqls-server/sqls/internal/database"
	"github.com/sqls-server/sqls/internal/diagnostic"
	"github.com/sqls-server/sqls/internal/lintconfig"
)

func predicateCache() *database.DBCache {
	return &database.DBCache{
		SchemaTables: map[string][]string{"": {"users"}},
		ColumnsWithParent: map[string][]*database.ColumnDesc{
			"\tUSERS": {
				{ColumnBase: database.ColumnBase{Table: "users", Name: "id"}, Type: "integer"},
				{ColumnBase: database.ColumnBase{Table: "users", Name: "name"}, Type: "text"},
				{ColumnBase: database.ColumnBase{Table: "users", Name: "email"}, Type: "text"},
				{ColumnBase: database.ColumnBase{Table: "users", Name: "active"}, Type: "boolean"},
				{ColumnBase: database.ColumnBase{Table: "users", Name: "custom_flag"}, Type: "boolean_domain"},
			},
		},
	}
}

func TestPredicateDiagnostics(t *testing.T) {
	for _, tc := range []struct {
		name, driver, sql string
		code              diagnostic.DiagnosticCode
		count             int
	}{
		{"reported query postgres", "postgresql", "SELECT u.name, u.email\nFROM users AS u\nWHERE u.id ;", diagnostic.CodeTypeMismatch, 1},
		{"reported query sqlite", "sqlite3", "SELECT u.name, u.email\nFROM users AS u\nWHERE u.id ;", diagnostic.CodeTypeMismatch, 0},
		{"mysql numeric truth", "mysql", "SELECT name FROM users WHERE id;", diagnostic.CodeTypeMismatch, 0},
		{"unknown dialect", "", "SELECT name FROM users WHERE id;", diagnostic.CodeTypeMismatch, 0},
		{"unqualified integer", "postgresql", "SELECT name FROM users WHERE id;", diagnostic.CodeTypeMismatch, 1},
		{"parenthesized integer", "postgresql", "SELECT name FROM users WHERE ((id));", diagnostic.CodeTypeMismatch, 1},
		{"negated integer", "postgresql", "SELECT name FROM users WHERE NOT id;", diagnostic.CodeTypeMismatch, 1},
		{"logical integer operand", "postgresql", "SELECT name FROM users WHERE active AND id;", diagnostic.CodeTypeMismatch, 1},
		{"logical comparison and integer", "postgresql", "SELECT name FROM users WHERE id = 1 OR id;", diagnostic.CodeTypeMismatch, 1},
		{"boolean column", "postgresql", "SELECT name FROM users WHERE active;", diagnostic.CodeTypeMismatch, 0},
		{"boolean expression", "postgresql", "SELECT name FROM users WHERE NOT active OR id = 1;", diagnostic.CodeTypeMismatch, 0},
		{"boolean literal", "postgresql", "SELECT name FROM users WHERE TRUE;", diagnostic.CodeTypeMismatch, 0},
		{"boolean string literal", "postgresql", "SELECT name FROM users WHERE 'true';", diagnostic.CodeTypeMismatch, 0},
		{"unknown function type", "postgresql", "SELECT name FROM users WHERE custom_function(id);", diagnostic.CodeTypeMismatch, 0},
		{"unknown domain", "postgresql", "SELECT name FROM users WHERE custom_flag;", diagnostic.CodeTypeMismatch, 0},
		{"boolean cast", "postgresql", "SELECT name FROM users WHERE id::boolean;", diagnostic.CodeTypeMismatch, 0},
		{"nested integer predicate", "postgresql", "SELECT name FROM users WHERE EXISTS (SELECT 1 FROM users AS u WHERE u.id);", diagnostic.CodeTypeMismatch, 1},
		{"having integer", "postgresql", "SELECT id FROM users GROUP BY id HAVING id;", diagnostic.CodeTypeMismatch, 1},
		{"empty where", "sqlite3", "SELECT name FROM users WHERE;", diagnostic.CodeSyntaxError, 1},
		{"empty where before order", "sqlite3", "SELECT name FROM users WHERE ORDER BY name;", diagnostic.CodeSyntaxError, 1},
		{"missing comparison operand", "sqlite3", "SELECT name FROM users WHERE id = ;", diagnostic.CodeSyntaxError, 1},
		{"parenthesized incomplete comparison", "sqlite3", "SELECT name FROM users WHERE (id =);", diagnostic.CodeSyntaxError, 1},
		{"incomplete logical operand", "sqlite3", "SELECT name FROM users WHERE id = 1 AND;", diagnostic.CodeSyntaxError, 1},
		{"incomplete logical nested operand", "sqlite3", "SELECT name FROM users WHERE id = 1 AND (id >);", diagnostic.CodeSyntaxError, 1},
		{"incomplete not", "sqlite3", "SELECT name FROM users WHERE NOT;", diagnostic.CodeSyntaxError, 1},
		{"incomplete is not", "sqlite3", "SELECT name FROM users WHERE id IS NOT;", diagnostic.CodeSyntaxError, 1},
		{"valid between", "postgresql", "SELECT name FROM users WHERE id BETWEEN 1 AND 2 AND active;", diagnostic.CodeSyntaxError, 0},
		{"valid between types", "postgresql", "SELECT name FROM users WHERE id BETWEEN 1 AND 2;", diagnostic.CodeTypeMismatch, 0},
		{"valid case", "postgresql", "SELECT name FROM users WHERE CASE WHEN active AND id = 1 THEN TRUE ELSE FALSE END;", diagnostic.CodeTypeMismatch, 0},
		{"valid count star", "sqlite3", "SELECT name FROM users WHERE (SELECT COUNT(*) FROM users);", diagnostic.CodeSyntaxError, 0},
		{"quoted keyword", "sqlite3", "SELECT name FROM users WHERE \"WHERE\";", diagnostic.CodeSyntaxError, 0},
		{"multiple statements", "sqlite3", "SELECT 1 WHERE; SELECT 2 WHERE;", diagnostic.CodeSyntaxError, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			diagnostics, err := New(nil, predicateCache(), nil, tc.driver).Lint(tc.sql)
			if err != nil {
				t.Fatal(err)
			}
			count := 0
			for _, d := range diagnostics {
				if d.Code == tc.code {
					count++
					if d.Severity != diagnostic.SeverityError {
						t.Errorf("expected an error: %+v", d)
					}
				}
			}
			if count != tc.count {
				t.Errorf("want %d %s diagnostics, got %d: %+v", tc.count, tc.code, count, diagnostics)
			}
		})
	}
}

func TestPredicateDiagnosticRange(t *testing.T) {
	diagnostics, _ := New(nil, predicateCache(), nil, "postgresql").Lint("SELECT u.name, u.email\nFROM users AS u\nWHERE u.id ;")
	for _, d := range diagnostics {
		if d.Code == diagnostic.CodeTypeMismatch {
			want := diagnostic.Range{Start: diagnostic.Position{Line: 2, Character: 6}, End: diagnostic.Position{Line: 2, Character: 10}}
			if d.Range != want {
				t.Fatalf("range = %+v; want %+v", d.Range, want)
			}
			return
		}
	}
	t.Fatal("missing predicate diagnostic")
}

func TestPredicateConfigurationAndDatabaseRefresh(t *testing.T) {
	config := lintconfig.DefaultConfig()
	config.CheckSyntax = false
	linter := New(config, predicateCache(), nil, "postgresql")
	for _, sql := range []string{"SELECT name FROM users WHERE id;", "SELECT name FROM users WHERE id =;"} {
		diagnostics, _ := linter.Lint(sql)
		for _, d := range diagnostics {
			if d.Code == diagnostic.CodeTypeMismatch || d.Code == diagnostic.CodeSyntaxError {
				t.Fatalf("disabled check reported %+v", d)
			}
		}
	}
	linter.UpdateConfig(lintconfig.DefaultConfig())
	linter.UpdateDatabase(predicateCache(), "sqlite3")
	diagnostics, _ := linter.Lint("SELECT name FROM users WHERE id;")
	if len(diagnostics) != 0 {
		t.Fatalf("SQLite predicate must be accepted: %+v", diagnostics)
	}
	linter.UpdateDatabase(predicateCache(), "postgresql")
	diagnostics, _ = linter.Lint("SELECT name FROM users WHERE id;")
	if len(diagnostics) != 1 || diagnostics[0].Code != diagnostic.CodeTypeMismatch {
		t.Fatalf("PostgreSQL predicate must be rejected: %+v", diagnostics)
	}
}
