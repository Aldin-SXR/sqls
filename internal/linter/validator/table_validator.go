package validator

import (
    "strings"

    "github.com/sqls-server/sqls/ast"
    "github.com/sqls-server/sqls/internal/database"
    "github.com/sqls-server/sqls/internal/diagnostic"
    "github.com/sqls-server/sqls/internal/lintconfig"
    "github.com/sqls-server/sqls/parser"
    "github.com/sqls-server/sqls/parser/parseutil"
    "github.com/sqls-server/sqls/token"
)

// TableValidator validates table references
type TableValidator struct {
    config  *lintconfig.Config
    dbCache *database.DBCache
}

// NewTableValidator creates a new table validator
func NewTableValidator(config *lintconfig.Config, dbCache *database.DBCache) *TableValidator {
	return &TableValidator{
		config:  config,
		dbCache: dbCache,
	}
}

// Validate performs table validation
func (v *TableValidator) Validate(text string, db *diagnostic.DiagnosticBuilder) {
    if !v.config.CheckTableReferences {
        return
    }
    if v.dbCache == nil {
        return
    }
    parsed, err := parser.Parse(text)
    if err != nil {
        return
    }
    // Gather potential table reference nodes across the statement
    nodes := []ast.Node{}
    nodes = append(nodes, parseutil.ExtractTableReferences(parsed)...)
    nodes = append(nodes, parseutil.ExtractTableReference(parsed)...)
    nodes = append(nodes, parseutil.ExtractTableFactor(parsed)...)

    for _, n := range nodes {
        v.validateNodeAsTable(n, db)
    }

    // Optionally warn for implicit joins (comma-separated tables)
    if v.config.WarnOnImplicitJoin {
        v.CheckImplicitJoins(text, db)
    }
}

// validateTableReference validates a single table reference
func (v *TableValidator) validateTableReference(schemaName, tableName string, startPos, endPos token.Pos, db *diagnostic.DiagnosticBuilder) {
    if tableName == "" && schemaName == "" {
        return
    }

    // Skip validation for very short identifiers that might be partial keywords being typed
    // This prevents false positives when user is typing "JOIN", "WHERE", etc.
    if len(tableName) <= 4 && v.mightBePartialKeyword(tableName) {
        return
    }

    // Validate schema first if present
    if schemaName != "" && !v.schemaExists(schemaName) {
        db.AddError(
            startPos,
            endPos,
            diagnostic.CodeInvalidSchema,
            diagnostic.FormatError(diagnostic.CodeInvalidSchema, schemaName),
        )
        return
    }
    if !v.tableExists(tableName, schemaName) {
        db.AddError(
            startPos,
            endPos,
            diagnostic.CodeTableNotFound,
            diagnostic.FormatError(diagnostic.CodeTableNotFound, v.formatTableName(schemaName, tableName)),
        )
    }
}

// mightBePartialKeyword checks if a short string could be a partial SQL keyword
func (v *TableValidator) mightBePartialKeyword(s string) bool {
    // Common SQL keywords that users might be typing
    keywords := []string{
        "JOIN", "LEFT", "RIGHT", "INNER", "OUTER", "CROSS", "FULL",
        "WHERE", "GROUP", "ORDER", "HAVING", "LIMIT", "OFFSET",
        "UNION", "INTERSECT", "EXCEPT",
        "AND", "OR", "NOT", "IN", "EXISTS", "LIKE", "BETWEEN",
        "AS", "ON", "USING", "WITH",
    }

    upper := strings.ToUpper(s)
    for _, kw := range keywords {
        if strings.HasPrefix(kw, upper) {
            return true
        }
    }
    return false
}

// validateNodeAsTable dispatches based on node type and validates
func (v *TableValidator) validateNodeAsTable(n ast.Node, db *diagnostic.DiagnosticBuilder) {
    switch t := n.(type) {
    case *ast.Identifier:
        v.validateTableReference("", t.NoQuoteString(), t.Pos(), t.End(), db)
    case *ast.MemberIdentifier:
        v.validateTableReference(t.GetParent().String(), t.GetChild().String(), t.Pos(), t.End(), db)
    case *ast.Aliased:
        switch real := t.RealName.(type) {
        case *ast.Identifier:
            v.validateTableReference("", real.NoQuoteString(), t.Pos(), t.End(), db)
        case *ast.MemberIdentifier:
            v.validateTableReference(real.GetParent().String(), real.GetChild().String(), t.Pos(), t.End(), db)
        case ast.TokenList:
            // subquery: skip
        }
    case *ast.IdentifierList:
        for _, id := range t.GetIdentifiers() {
            v.validateNodeAsTable(id, db)
        }
    }
}

// tokenPos converts a struct with Line/Col to token.Pos (duck-typed)
// token positions are provided directly as token.Pos

// tableExists checks if a table exists in the database cache
func (v *TableValidator) tableExists(tableName, schemaName string) bool {
	if v.dbCache == nil {
		return true // Assume exists if no cache
	}

	// If schema specified, check in that schema
	if schemaName != "" {
		if tables, ok := v.dbCache.SchemaTables[schemaName]; ok {
			for _, table := range tables {
				if strings.EqualFold(table, tableName) {
					return true
				}
			}
		}
		return false
	}

	// Check in default schema
	for _, table := range v.dbCache.SortedTables() {
		if strings.EqualFold(table, tableName) {
			return true
		}
	}

	// Check in all schemas if not found in default
	for _, tables := range v.dbCache.SchemaTables {
		for _, table := range tables {
			if strings.EqualFold(table, tableName) {
				return true
			}
		}
	}

	return false
}

// schemaExists checks if a schema exists in the database cache
func (v *TableValidator) schemaExists(schemaName string) bool {
	if v.dbCache == nil {
		return true // Assume exists if no cache
	}

	for _, schema := range v.dbCache.SortedSchemas() {
		if strings.EqualFold(schema, schemaName) {
			return true
		}
	}

	return false
}

// formatTableName formats schema.table or just table
func (v *TableValidator) formatTableName(schema, table string) string {
	if schema != "" {
		return schema + "." + table
	}
	return table
}

// GetAvailableTables returns all available tables for a schema
func (v *TableValidator) GetAvailableTables(schema string) []string {
	if v.dbCache == nil {
		return nil
	}

	if schema != "" {
		if tables, ok := v.dbCache.SchemaTables[schema]; ok {
			return tables
		}
		return nil
	}

	return v.dbCache.SortedTables()
}

// GetTableInfo returns information about a table
func (v *TableValidator) GetTableInfo(tableName, schemaName string) ([]*database.ColumnDesc, bool) {
	if v.dbCache == nil {
		return nil, false
	}

	// Try to get columns for the table
	fullName := v.formatTableName(schemaName, tableName)
	cols, ok := v.dbCache.ColumnDescs(fullName)
	if ok {
		return cols, true
	}

	// Try with just table name
	cols, ok = v.dbCache.ColumnDescs(tableName)
	return cols, ok
}

// ExtractTablesFromQuery: no longer needed; use parseutil directly where required

// CheckImplicitJoins checks for implicit joins (comma-separated tables in FROM)
func (v *TableValidator) CheckImplicitJoins(text string, db *diagnostic.DiagnosticBuilder) {
    parsed, err := parser.Parse(text)
    if err != nil {
        return
    }
    toks := flattenTokens(parsed)
    inFrom := false
    var lastComma *ast.SQLToken
    for i := 0; i < len(toks); i++ {
        t := toks[i]
        if t.Kind == token.SQLKeyword {
            if w, ok := t.Value.(*token.SQLWord); ok {
                up := strings.ToUpper(w.Keyword)
                switch up {
                case "FROM":
                    inFrom = true
                    lastComma = nil
                case "JOIN", "WHERE", "ON":
                    inFrom = false
                }
            }
            continue
        }
        if inFrom && t.Kind == token.Comma {
            lastComma = t
        }
    }
    if lastComma != nil {
        db.AddWarning(lastComma.From, lastComma.To, diagnostic.CodeImplicitJoin, "Implicit join detected, consider using explicit JOIN syntax")
    }
}
