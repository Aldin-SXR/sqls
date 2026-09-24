package linter

import (
	"testing"

	"github.com/sqls-server/sqls/internal/database"
	"github.com/sqls-server/sqls/internal/diagnostic"
	"github.com/sqls-server/sqls/internal/lintconfig"
)

func TestLintRules(t *testing.T) {
	cache := &database.DBCache{
		Schemas:      map[string]string{"PUBLIC": "public"},
		SchemaTables: map[string][]string{"PUBLIC": {"customers", "orders"}},
	}
	for _, tc := range []struct {
		name, sql string
		code      diagnostic.DiagnosticCode
		count     int
	}{
		{"qualified table", "SELECT id FROM public.customers;", diagnostic.CodeTableNotFound, 0},
		{"mixed case schema", "SELECT id FROM Public.customers;", diagnostic.CodeTableNotFound, 0},
		{"unknown table", "SELECT id FROM public.missing_table;", diagnostic.CodeTableNotFound, 1},
		{"implicit join once", "SELECT customers.id FROM customers, orders;", diagnostic.CodeImplicitJoin, 1},
		{"order by comma", "SELECT id FROM customers ORDER BY id, name;", diagnostic.CodeImplicitJoin, 0},
		{"group by comma", "SELECT id FROM customers GROUP BY id, name;", diagnostic.CodeImplicitJoin, 0},
		{"explicit join", "SELECT c.id FROM customers c JOIN orders o ON c.id = o.id;", diagnostic.CodeImplicitJoin, 0},
		{"two statements", "SELECT id FROM customers, orders; SELECT id FROM customers, orders;", diagnostic.CodeImplicitJoin, 2},
		{"nested implicit join", "SELECT id FROM (SELECT id FROM customers, orders) q;", diagnostic.CodeImplicitJoin, 1},
		{"multiplication", "SELECT 2 * 3;", diagnostic.CodeSelectStar, 0},
		{"count star", "SELECT COUNT(*) FROM customers;", diagnostic.CodeSelectStar, 0},
		{"wildcard", "SELECT * FROM customers;", diagnostic.CodeSelectStar, 1},
		{"qualified wildcard", "SELECT customers.* FROM customers;", diagnostic.CodeSelectStar, 1},
		{"distinct wildcard", "SELECT DISTINCT * FROM customers;", diagnostic.CodeSelectStar, 1},
		{"mixed expressions", "SELECT 2 * 3, customers.* FROM customers;", diagnostic.CodeSelectStar, 1},
		{"nested wildcard", "SELECT id FROM (SELECT * FROM customers) q;", diagnostic.CodeSelectStar, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := lintconfig.DefaultConfig()
			cfg.CheckColumnReferences = false
			ds, err := New(cfg, cache, nil, "postgresql").Lint(tc.sql)
			if err != nil {
				t.Fatal(err)
			}
			count := 0
			for _, d := range ds {
				if d.Code == tc.code {
					count++
				}
			}
			if count != tc.count {
				t.Fatalf("wanted %d %s diagnostics, got %d: %+v", tc.count, tc.code, count, ds)
			}
		})
	}
}

func TestParseFailureDiagnostic(t *testing.T) {
	ds, err := New(nil, nil, nil, "").Lint("SELECT /* unterminated")
	if err != nil {
		t.Fatalf("SQL errors must be diagnostics, not transport errors: %v", err)
	}
	if len(ds) != 1 || ds[0].Code != diagnostic.CodeSyntaxError {
		t.Fatalf("expected syntax diagnostic: %+v", ds)
	}
}
