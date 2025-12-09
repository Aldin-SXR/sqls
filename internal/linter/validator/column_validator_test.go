package validator

import (
	"strings"
	"testing"

	"github.com/sqls-server/sqls/internal/database"
	"github.com/sqls-server/sqls/internal/diagnostic"
	"github.com/sqls-server/sqls/internal/lintconfig"
	"github.com/sqls-server/sqls/parser"
)

func TestColumnAliasInOrderBy(t *testing.T) {
	// Create a mock database cache
	dbCache := &database.DBCache{
		Schemas:      map[string]string{"": ""},
		SchemaTables: map[string][]string{"": {"CUSTOMERS"}},
		ColumnsWithParent: map[string][]*database.ColumnDesc{
			"\tCUSTOMERS": {
				{ColumnBase: database.ColumnBase{Schema: "", Table: "customers", Name: "id"}, Type: "int"},
				{ColumnBase: database.ColumnBase{Schema: "", Table: "customers", Name: "customer_name"}, Type: "varchar"},
				{ColumnBase: database.ColumnBase{Schema: "", Table: "customers", Name: "email"}, Type: "varchar"},
			},
		},
		ForeignKeys: map[string]map[string][]*database.ForeignKey{},
	}

	config := &lintconfig.Config{
		Enabled:              true,
		CheckColumnReferences: true,
	}

	validator := NewColumnValidator(config, dbCache, "mysql")

	tests := []struct {
		name        string
		sql         string
		shouldError bool
		description string
	}{
		{
			name:        "alias in ORDER BY with AS keyword",
			sql:         "SELECT customer_name AS cn FROM customers ORDER BY cn",
			shouldError: false,
			description: "Should NOT error when using column alias in ORDER BY",
		},
		{
			name:        "alias in ORDER BY without AS keyword",
			sql:         "SELECT customer_name cn FROM customers ORDER BY cn",
			shouldError: false,
			description: "Should NOT error when using column alias (without AS) in ORDER BY",
		},
		{
			name:        "multiple aliases in ORDER BY",
			sql:         "SELECT customer_name AS cn, email AS em FROM customers ORDER BY cn, em",
			shouldError: false,
			description: "Should NOT error when using multiple aliases in ORDER BY",
		},
		{
			name:        "alias in HAVING",
			sql:         "SELECT COUNT(*) AS cnt FROM customers GROUP BY customer_name HAVING cnt > 5",
			shouldError: false,
			description: "Should NOT error when using alias in HAVING clause",
		},
		{
			name:        "non-existent column in ORDER BY",
			sql:         "SELECT customer_name FROM customers ORDER BY nonexistent",
			shouldError: true,
			description: "Should error when using non-existent column in ORDER BY",
		},
		{
			name:        "real column in ORDER BY",
			sql:         "SELECT customer_name FROM customers ORDER BY customer_name",
			shouldError: false,
			description: "Should NOT error when using real column in ORDER BY",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Validate that SQL can be parsed
			_, err := parser.Parse(tt.sql)
			if err != nil {
				t.Fatalf("Failed to parse SQL: %v", err)
			}

			db := diagnostic.NewDiagnosticBuilder()
			validator.Validate(tt.sql, db)
			diagnostics := db.Build()

			hasError := false
			for _, d := range diagnostics {
				if d.Severity == diagnostic.SeverityError &&
				   (d.Code == diagnostic.CodeColumnNotFound ||
				    strings.Contains(d.Message, "not found")) {
					hasError = true
					t.Logf("Error found: %s", d.Message)
				}
			}

			if tt.shouldError && !hasError {
				t.Errorf("%s: Expected error but got none", tt.description)
			}
			if !tt.shouldError && hasError {
				t.Errorf("%s: Expected no error but got error", tt.description)
			}
		})
	}
}

func TestExtractSelectColumnAliases(t *testing.T) {
	config := &lintconfig.Config{
		Enabled: true,
	}
	validator := NewColumnValidator(config, nil, "mysql")

	tests := []struct {
		name            string
		sql             string
		expectedAliases []string
	}{
		{
			name:            "single alias with AS",
			sql:             "SELECT customer_name AS cn FROM customers",
			expectedAliases: []string{"cn"},
		},
		{
			name:            "single alias without AS",
			sql:             "SELECT customer_name cn FROM customers",
			expectedAliases: []string{"cn"},
		},
		{
			name:            "multiple aliases",
			sql:             "SELECT customer_name AS cn, email AS em FROM customers",
			expectedAliases: []string{"cn", "em"},
		},
		{
			name:            "mixed case aliases",
			sql:             "SELECT customer_name AS CustName, email AS EMAIL FROM customers",
			expectedAliases: []string{"custname", "email"},
		},
		{
			name:            "no aliases",
			sql:             "SELECT customer_name, email FROM customers",
			expectedAliases: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := parser.Parse(tt.sql)
			if err != nil {
				t.Fatalf("Failed to parse SQL: %v", err)
			}

			aliases := validator.extractSelectColumnAliases(parsed)

			// Check that all expected aliases are present
			for _, expected := range tt.expectedAliases {
				expectedLower := strings.ToLower(expected)
				if !aliases[expectedLower] {
					t.Errorf("Expected alias '%s' not found in result", expected)
				}
			}

			// Check that no unexpected aliases are present
			if len(aliases) != len(tt.expectedAliases) {
				t.Errorf("Expected %d aliases but got %d", len(tt.expectedAliases), len(aliases))
			}
		})
	}
}

func TestSubQueryTableReferences(t *testing.T) {
	// Create a mock database cache with all columns needed for tests
	dbCache := &database.DBCache{
		Schemas:      map[string]string{"": ""},
		SchemaTables: map[string][]string{"": {"CUSTOMERS", "ORDERS"}},
		ColumnsWithParent: map[string][]*database.ColumnDesc{
			"\tCUSTOMERS": {
				{ColumnBase: database.ColumnBase{Schema: "", Table: "customers", Name: "id"}, Type: "int"},
				{ColumnBase: database.ColumnBase{Schema: "", Table: "customers", Name: "customer_id"}, Type: "int"},
				{ColumnBase: database.ColumnBase{Schema: "", Table: "customers", Name: "customer_name"}, Type: "varchar"},
				{ColumnBase: database.ColumnBase{Schema: "", Table: "customers", Name: "email"}, Type: "varchar"},
			},
			"\tORDERS": {
				{ColumnBase: database.ColumnBase{Schema: "", Table: "orders", Name: "id"}, Type: "int"},
				{ColumnBase: database.ColumnBase{Schema: "", Table: "orders", Name: "customer_id"}, Type: "int"},
				{ColumnBase: database.ColumnBase{Schema: "", Table: "orders", Name: "order_date"}, Type: "date"},
			},
		},
		ForeignKeys: map[string]map[string][]*database.ForeignKey{},
	}

	config := &lintconfig.Config{
		Enabled:               true,
		CheckColumnReferences: true,
	}

	validator := NewColumnValidator(config, dbCache, "mysql")

	tests := []struct {
		name        string
		sql         string
		shouldError bool
		description string
	}{
		{
			name:        "subquery in WHERE IN clause",
			sql:         "SELECT * FROM customers WHERE id IN (SELECT customer_id FROM orders)",
			shouldError: false,
			description: "Should NOT error on table name in subquery FROM clause",
		},
		{
			name:        "same table in outer and inner query",
			sql:         "SELECT * FROM customers WHERE id IN (SELECT id FROM customers)",
			shouldError: false,
			description: "Should NOT error when same table appears in both outer and inner query",
		},
		{
			name:        "nested subqueries",
			sql:         "SELECT * FROM customers WHERE id IN (SELECT customer_id FROM orders WHERE id IN (SELECT id FROM customers))",
			shouldError: false,
			description: "Should NOT error on table names in nested subqueries",
		},
		{
			name:        "subquery in FROM clause (derived table)",
			sql:         "SELECT * FROM (SELECT * FROM customers) AS c",
			shouldError: false,
			description: "Should NOT error on table name in FROM clause subquery",
		},
		{
			name:        "multiple subqueries in same query",
			sql:         "SELECT * FROM customers WHERE id IN (SELECT customer_id FROM orders) AND email IN (SELECT email FROM customers WHERE id > 10)",
			shouldError: false,
			description: "Should NOT error on table names in multiple subqueries",
		},
		{
			name:        "correlated subquery",
			sql:         "SELECT * FROM customers WHERE EXISTS (SELECT 1 FROM orders WHERE customer_id = id)",
			shouldError: false,
			description: "Should NOT error on table names in correlated subquery",
		},
		{
			name:        "subquery with simple table reference",
			sql:         "SELECT * FROM customers WHERE id IN (SELECT customer_id FROM orders)",
			shouldError: false,
			description: "Should NOT error on table names in subquery",
		},
		{
			name:        "actual table name misuse",
			sql:         "SELECT customers FROM customers",
			shouldError: true,
			description: "Should still error when table name is incorrectly used as column in outer query",
		},
		{
			name:        "non-existent table in subquery",
			sql:         "SELECT * FROM customers WHERE id IN (SELECT id FROM nonexistent)",
			shouldError: false,
			description: "Should NOT error on table name in subquery (table existence is not a column validation error)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Validate that SQL can be parsed
			_, err := parser.Parse(tt.sql)
			if err != nil {
				t.Fatalf("Failed to parse SQL: %v", err)
			}

			db := diagnostic.NewDiagnosticBuilder()
			validator.Validate(tt.sql, db)
			diagnostics := db.Build()

			hasError := false
			for _, d := range diagnostics {
				if d.Severity == diagnostic.SeverityError {
					hasError = true
					t.Logf("Error found: %s", d.Message)
				}
			}

			if tt.shouldError && !hasError {
				t.Errorf("%s: Expected error but got none", tt.description)
			}
			if !tt.shouldError && hasError {
				t.Errorf("%s: Expected no error but got error", tt.description)
			}
		})
	}
}

func TestSubQueryWithDifferentPositions(t *testing.T) {
	dbCache := &database.DBCache{
		Schemas:      map[string]string{"": ""},
		SchemaTables: map[string][]string{"": {"CUSTOMERS", "ORDERS", "TICKETS", "SHOWTIMES", "MOVIES"}},
		ColumnsWithParent: map[string][]*database.ColumnDesc{
			"\tCUSTOMERS": {
				{ColumnBase: database.ColumnBase{Schema: "", Table: "customers", Name: "id"}, Type: "int"},
				{ColumnBase: database.ColumnBase{Schema: "", Table: "customers", Name: "customer_id"}, Type: "int"},
				{ColumnBase: database.ColumnBase{Schema: "", Table: "customers", Name: "customer_name"}, Type: "varchar"},
				{ColumnBase: database.ColumnBase{Schema: "", Table: "customers", Name: "full_name"}, Type: "varchar"},
			},
			"\tORDERS": {
				{ColumnBase: database.ColumnBase{Schema: "", Table: "orders", Name: "id"}, Type: "int"},
				{ColumnBase: database.ColumnBase{Schema: "", Table: "orders", Name: "customer_id"}, Type: "int"},
			},
			"\tTICKETS": {
				{ColumnBase: database.ColumnBase{Schema: "", Table: "tickets", Name: "id"}, Type: "int"},
				{ColumnBase: database.ColumnBase{Schema: "", Table: "tickets", Name: "customer_id"}, Type: "int"},
				{ColumnBase: database.ColumnBase{Schema: "", Table: "tickets", Name: "showtime_id"}, Type: "int"},
			},
			"\tSHOWTIMES": {
				{ColumnBase: database.ColumnBase{Schema: "", Table: "showtimes", Name: "id"}, Type: "int"},
				{ColumnBase: database.ColumnBase{Schema: "", Table: "showtimes", Name: "movie_id"}, Type: "int"},
			},
			"\tMOVIES": {
				{ColumnBase: database.ColumnBase{Schema: "", Table: "movies", Name: "id"}, Type: "int"},
				{ColumnBase: database.ColumnBase{Schema: "", Table: "movies", Name: "genre_id"}, Type: "int"},
			},
		},
		ForeignKeys: map[string]map[string][]*database.ForeignKey{},
	}

	config := &lintconfig.Config{
		Enabled:               true,
		CheckColumnReferences: true,
	}

	validator := NewColumnValidator(config, dbCache, "mysql")

	tests := []struct {
		name        string
		sql         string
		shouldError bool
	}{
		{
			name:        "subquery in HAVING clause",
			sql:         "SELECT customer_name, COUNT(*) as cnt FROM customers GROUP BY customer_name HAVING cnt > (SELECT AVG(customer_id) FROM orders)",
			shouldError: false,
		},
		{
			name:        "union with subqueries",
			sql:         "SELECT id FROM (SELECT id FROM customers) AS c UNION SELECT id FROM (SELECT customer_id as id FROM orders) AS o",
			shouldError: false,
		},
		{
			name:        "subquery with JOIN",
			sql:         "SELECT id FROM customers WHERE id IN (SELECT customer_id FROM orders)",
			shouldError: false,
		},
		{
			name:        "qualified reference WITHOUT subquery",
			sql:         "SELECT t.id FROM tickets t",
			shouldError: false,
		},
		{
			name:        "simple subquery with qualified table reference",
			sql:         "SELECT id FROM customers WHERE id IN (SELECT t.id FROM tickets t)",
			shouldError: false,
		},
		{
			name:        "derived table with alias",
			sql:         "SELECT sub.id FROM (SELECT t.id FROM tickets t) AS sub",
			shouldError: false,
		},
		{
			name:        "user complex query with qualified names and JOINs",
			sql:         "SELECT c.id, c.full_name FROM customers c WHERE c.id IN (SELECT t.id FROM tickets t JOIN showtimes s ON t.showtime_id = s.id JOIN movies m ON s.movie_id = m.id GROUP BY t.customer_id HAVING COUNT(DISTINCT m.genre_id) > 1)",
			shouldError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parser.Parse(tt.sql)
			if err != nil {
				t.Fatalf("Failed to parse SQL: %v", err)
			}

			db := diagnostic.NewDiagnosticBuilder()
			validator.Validate(tt.sql, db)
			diagnostics := db.Build()

			hasError := false
			for _, d := range diagnostics {
				if d.Severity == diagnostic.SeverityError {
					hasError = true
					t.Logf("Error found: %s", d.Message)
				}
			}

			if tt.shouldError && !hasError {
				t.Errorf("Expected error but got none")
			}
			if !tt.shouldError && hasError {
				t.Errorf("Expected no error but got error")
			}
		})
	}
}
