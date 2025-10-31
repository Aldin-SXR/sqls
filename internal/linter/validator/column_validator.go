package validator

import (
    "fmt"
    "strings"

    "github.com/sqls-server/sqls/ast"
    "github.com/sqls-server/sqls/internal/database"
    "github.com/sqls-server/sqls/internal/diagnostic"
    "github.com/sqls-server/sqls/internal/lintconfig"
    "github.com/sqls-server/sqls/parser"
    "github.com/sqls-server/sqls/parser/parseutil"
    "github.com/sqls-server/sqls/token"
)

// ColumnValidator validates column references
type ColumnValidator struct {
    config  *lintconfig.Config
    dbCache *database.DBCache
    driver  string // Database driver (e.g., "mysql", "postgresql")
}

// NewColumnValidator creates a new column validator
func NewColumnValidator(config *lintconfig.Config, dbCache *database.DBCache, driver string) *ColumnValidator {
	return &ColumnValidator{
		config:  config,
		dbCache: dbCache,
		driver:  driver,
	}
}

// Validate performs column validation
func (v *ColumnValidator) Validate(text string, db *diagnostic.DiagnosticBuilder) {
    if !v.config.CheckColumnReferences {
        return
    }
    if v.dbCache == nil {
        return
    }
    parsed, err := parser.Parse(text)
    if err != nil {
        return
    }

    // Build table list and alias map alias->table
    aliasMap := map[string]string{}
    tables := v.extractTables(parsed, aliasMap)
    ctx := v.buildColumnContext(tables)

    // FIRST: Collect all identifiers that should be skipped from column validation
    // This includes:
    // 1. Identifiers that are part of MemberIdentifier nodes (qualified references like "customers.id")
    // 2. Identifiers that are table references (FROM/JOIN clauses like "FROM customers")
    // Use position-based tracking instead of pointer comparison to avoid instance mismatch issues
    skipIdentifierPositions := make(map[string]bool)

    // Collect MemberIdentifier components
    walk(parsed, func(n ast.Node) {
        if m, ok := n.(*ast.MemberIdentifier); ok {
            if m.ParentIdent != nil {
                pos := fmt.Sprintf("%d:%d", m.ParentIdent.Pos().Line, m.ParentIdent.Pos().Col)
                skipIdentifierPositions[pos] = true // Mark parent (table/alias name)
            }
            if m.ChildIdent != nil {
                pos := fmt.Sprintf("%d:%d", m.ChildIdent.Pos().Line, m.ChildIdent.Pos().Col)
                skipIdentifierPositions[pos] = true // Mark child (column name)
            }
        }
    })

    // Collect table reference identifiers (FROM/JOIN clauses)
    var collectTableRefPositions func(ast.Node)
    collectTableRefPositions = func(n ast.Node) {
        if n == nil {
            return
        }
        switch t := n.(type) {
        case *ast.Identifier:
            pos := fmt.Sprintf("%d:%d", t.Pos().Line, t.Pos().Col)
            skipIdentifierPositions[pos] = true
        case *ast.MemberIdentifier:
            // Schema.table references
            if t.ChildIdent != nil {
                pos := fmt.Sprintf("%d:%d", t.ChildIdent.Pos().Line, t.ChildIdent.Pos().Col)
                skipIdentifierPositions[pos] = true
            }
            if t.ParentIdent != nil {
                pos := fmt.Sprintf("%d:%d", t.ParentIdent.Pos().Line, t.ParentIdent.Pos().Col)
                skipIdentifierPositions[pos] = true
            }
        case *ast.Aliased:
            // Table aliases: "FROM customers AS c" - skip both "customers" and position of alias itself
            collectTableRefPositions(t.RealName)
        case *ast.IdentifierList:
            // Multiple table references: "FROM table1, table2" or JOIN chains
            for _, id := range t.GetIdentifiers() {
                collectTableRefPositions(id)
            }
        }
    }

    // Collect from all table reference extraction points
    for _, node := range parseutil.ExtractTableReferences(parsed) {
        collectTableRefPositions(node)
    }
    for _, node := range parseutil.ExtractTableReference(parsed) {
        collectTableRefPositions(node)
    }
    for _, node := range parseutil.ExtractTableFactor(parsed) {
        collectTableRefPositions(node)
    }

    // Collect alias names from Aliased nodes (e.g., "SELECT col AS alias_name")
    // The alias names themselves should not be validated as column references
    walk(parsed, func(n ast.Node) {
        if aliased, ok := n.(*ast.Aliased); ok {
            if aliased.AliasedName != nil {
                // Walk the aliased name to find all identifiers within it
                walk(aliased.AliasedName, func(aliasNode ast.Node) {
                    if id, ok := aliasNode.(*ast.Identifier); ok {
                        pos := fmt.Sprintf("%d:%d", id.Pos().Line, id.Pos().Col)
                        skipIdentifierPositions[pos] = true
                    }
                })
            }
        }
    })

    // Validate qualified column references (t.col and t.*)
    walk(parsed, func(n ast.Node) {
        m, ok := n.(*ast.MemberIdentifier)
        if !ok || m.ChildIdent == nil {
            return
        }

        // Parent might be alias or table name
        parent := m.ParentIdent
        if parent == nil {
            return
        }
        parentName := parent.NoQuoteString()
        tableName := parentName
        isValidAlias := false
        if t, ok := aliasMap[strings.ToLower(parentName)]; ok {
            tableName = t
            isValidAlias = true
        }

        // Check if parentName references a valid table/alias from the query
        // If it's not an alias, check if it's a table name in the context
        if !isValidAlias {
            _, foundInContext := ctx.TableColumns[strings.ToLower(parentName)]
            if !foundInContext {
                // Check if it's a valid table name from the tables list
                isValidTable := false
                for _, tableInfo := range tables {
                    if strings.EqualFold(tableInfo.Name, parentName) || strings.EqualFold(tableInfo.Alias, parentName) {
                        isValidTable = true
                        break
                    }
                }
                if !isValidTable {
                    // Invalid table/alias reference
                    db.AddError(
                        parent.Pos(),
                        parent.End(),
                        diagnostic.CodeTableNotFound,
                        fmt.Sprintf("Table or alias '%s' not found in query", parentName),
                    )
                    return
                }
            }
        }

        // Allow wildcard expansion like alias.* or table.*
        colName := m.ChildIdent.NoQuoteString()
        if m.ChildIdent.IsWildcard() || colName == "*" || colName == "" {
            return
        }

        // Look up columns from context (uses case-insensitive keys)
        cols, ok := ctx.TableColumns[strings.ToLower(tableName)]
        if !ok {
            // Try looking up from cache as fallback
            cols, ok = v.dbCache.ColumnDescs(tableName)
            if !ok {
                // search all schemas
                for _, schema := range v.dbCache.SortedSchemas() {
                    if c, ok2 := v.dbCache.ColumnDatabase(schema, tableName); ok2 {
                        cols, ok = c, true
                        break
                    }
                }
            }
        }

        if !ok || len(cols) == 0 {
            // If we can't find the table columns, don't report column errors
            // (the table exists in the query but we don't have schema info)
            return
        }
        found := false
        for _, c := range cols {
            if strings.EqualFold(c.Name, colName) {
                found = true
                break
            }
        }
        if !found {
            db.AddError(
                m.ChildIdent.Pos(),
                m.ChildIdent.End(),
                diagnostic.CodeColumnNotFound,
                diagnostic.FormatError(diagnostic.CodeColumnNotFound, colName, tableName),
            )
        }
    })

    // Validate unqualified identifiers in SELECT and WHERE
    // 1) SELECT list
    for _, node := range parseutil.ExtractSelectExpr(parsed) {
        walk(node, func(n ast.Node) {
            if id, ok := n.(*ast.Identifier); ok {
                // Skip if this identifier should not be validated as a column
                idPos := fmt.Sprintf("%d:%d", id.Pos().Line, id.Pos().Col)
                if skipIdentifierPositions[idPos] {
                    return
                }

                // Skip string literals (single or double-quoted strings)
                if v.isStringLiteral(id) {
                    return
                }

                name := id.NoQuoteString()
                if name == "" || id.IsWildcard() {
                    return
                }
                // Skip aliases and table names
                if _, ok := aliasMap[strings.ToLower(name)]; ok {
                    return
                }
                nameLower := strings.ToLower(name)
                if _, existsInAny := ctx.AllColumns[nameLower]; !existsInAny {
                    if len(ctx.TableColumns) > 0 && v.looksLikeColumnReference(id) {
                        db.AddError(id.Pos(), id.End(), diagnostic.CodeColumnNotFound, fmt.Sprintf("Column '%s' not found in any referenced table", name))
                    }
                    return
                }
                // Ambiguity check
                if cols := ctx.AllColumns[nameLower]; len(cols) > 1 && v.config.WarnOnAmbiguousColumn {
                    // Collect unique table names for message
                    seen := map[string]bool{}
                    unique := []string{}
                    for _, c := range cols {
                        if !seen[c.Table] {
                            seen[c.Table] = true
                            unique = append(unique, c.Table)
                        }
                    }
                    if len(unique) > 1 {
                        db.AddWarning(id.Pos(), id.End(), diagnostic.CodeAmbiguousColumn, diagnostic.FormatError(diagnostic.CodeAmbiguousColumn, name, strings.Join(unique, ", ")))
                    }
                }
            }
        })
    }
    // 2) WHERE conditions
    for _, node := range parseutil.ExtractWhereCondition(parsed) {
        walk(node, func(n ast.Node) {
            if id, ok := n.(*ast.Identifier); ok {
                // Skip if this identifier should not be validated as a column
                idPos := fmt.Sprintf("%d:%d", id.Pos().Line, id.Pos().Col)
                if skipIdentifierPositions[idPos] {
                    return
                }

                // Skip string literals (single or double-quoted strings)
                if v.isStringLiteral(id) {
                    return
                }

                name := id.NoQuoteString()
                if name == "" || id.IsWildcard() {
                    return
                }
                if _, ok := aliasMap[strings.ToLower(name)]; ok {
                    return
                }
                nameLower := strings.ToLower(name)
                if _, existsInAny := ctx.AllColumns[nameLower]; !existsInAny {
                    if len(ctx.TableColumns) > 0 && v.looksLikeColumnReference(id) {
                        db.AddError(id.Pos(), id.End(), diagnostic.CodeColumnNotFound, fmt.Sprintf("Column '%s' not found in any referenced table", name))
                    }
                    return
                }
                if cols := ctx.AllColumns[nameLower]; len(cols) > 1 && v.config.WarnOnAmbiguousColumn {
                    seen := map[string]bool{}
                    unique := []string{}
                    for _, c := range cols {
                        if !seen[c.Table] {
                            seen[c.Table] = true
                            unique = append(unique, c.Table)
                        }
                    }
                    if len(unique) > 1 {
                        db.AddWarning(id.Pos(), id.End(), diagnostic.CodeAmbiguousColumn, diagnostic.FormatError(diagnostic.CodeAmbiguousColumn, name, strings.Join(unique, ", ")))
                    }
                }
            }
        })
    }

    // 3) Validate standalone unqualified identifiers in the entire query
    // This catches identifiers in ON clauses, ORDER BY, etc. that aren't in SELECT/WHERE
    // skipIdentifierPositions already collected above, so just validate remaining identifiers
    walk(parsed, func(n ast.Node) {
        id, ok := n.(*ast.Identifier)
        if !ok {
            return
        }

        // Skip if this identifier should not be validated as a column
        idPos := fmt.Sprintf("%d:%d", id.Pos().Line, id.Pos().Col)
        if skipIdentifierPositions[idPos] {
            return
        }

        // Skip string literals (single or double-quoted strings)
        if v.isStringLiteral(id) {
            return
        }

        name := id.NoQuoteString()
        if name == "" || id.IsWildcard() {
            return
        }

        // Skip if it's a table alias (but NOT a table name - see below)
        if _, ok := aliasMap[strings.ToLower(name)]; ok {
            return
        }

        nameLower := strings.ToLower(name)

        // Check if column exists
        if _, existsInAny := ctx.AllColumns[nameLower]; !existsInAny {
            // Check if it's a known table name used incorrectly as a column
            isTableName := false
            for _, tableInfo := range tables {
                if strings.EqualFold(tableInfo.Name, name) {
                    isTableName = true
                    break
                }
            }

            if isTableName {
                // Error: table name used where column expected
                db.AddError(
                    id.Pos(),
                    id.End(),
                    diagnostic.CodeColumnNotFound,
                    fmt.Sprintf("'%s' is a table name, not a column. Did you mean '%s.column_name'?", name, name),
                )
            } else if len(ctx.TableColumns) > 0 && v.looksLikeColumnReference(id) {
                // Regular column not found error
                db.AddError(id.Pos(), id.End(), diagnostic.CodeColumnNotFound, fmt.Sprintf("Column '%s' not found in any referenced table", name))
            }
            return
        }

        // Ambiguity check - only for unqualified references
        if cols := ctx.AllColumns[nameLower]; len(cols) > 1 && v.config.WarnOnAmbiguousColumn {
            seen := map[string]bool{}
            unique := []string{}
            for _, c := range cols {
                if !seen[c.Table] {
                    seen[c.Table] = true
                    unique = append(unique, c.Table)
                }
            }
            if len(unique) > 1 {
                db.AddWarning(id.Pos(), id.End(), diagnostic.CodeAmbiguousColumn, diagnostic.FormatError(diagnostic.CodeAmbiguousColumn, name, strings.Join(unique, ", ")))
            }
        }
    })
}

// ColumnContext holds information about columns available in the query
type ColumnContext struct {
	// Map of table name -> columns
	TableColumns map[string][]*database.ColumnDesc
	// Map of table alias -> actual table name
	TableAliases map[string]string
	// All available columns (for unqualified references)
	AllColumns map[string][]*database.ColumnDesc // column name -> tables that have it
}

// buildColumnContext builds the column context from table references
func (v *ColumnValidator) buildColumnContext(tables []*parseutil.TableInfo) *ColumnContext {
	context := &ColumnContext{
		TableColumns: make(map[string][]*database.ColumnDesc),
		TableAliases: make(map[string]string),
		AllColumns:   make(map[string][]*database.ColumnDesc),
	}

	for _, tableInfo := range tables {
		tableName := tableInfo.Name
		alias := tableInfo.Alias

		// Get columns for this table
		cols, ok := v.dbCache.ColumnDescs(tableName)
		if !ok && tableInfo.DatabaseSchema != "" {
			// Try with schema-qualified name
			fullName := tableInfo.DatabaseSchema + "." + tableName
			cols, ok = v.dbCache.ColumnDescs(fullName)
		}
		// If still not found, search all schemas (important for JOIN tables)
		if !ok {
			for _, schema := range v.dbCache.SortedSchemas() {
				if c, found := v.dbCache.ColumnDatabase(schema, tableName); found {
					cols, ok = c, true
					break
				}
			}
		}

		if ok && len(cols) > 0 {
			// Store by table name for lookup (case-insensitive key)
			context.TableColumns[strings.ToLower(tableName)] = cols

			// Register alias (case-insensitive storage already handled in aliasMap)
			if alias != "" {
				context.TableAliases[strings.ToLower(alias)] = tableName
			}

			// Also register the table name itself as a valid reference
			context.TableAliases[strings.ToLower(tableName)] = tableName

			// Add to all columns map for ambiguity checking
			for _, col := range cols {
				colName := col.Name
				if existing, ok := context.AllColumns[strings.ToLower(colName)]; ok {
					context.AllColumns[strings.ToLower(colName)] = append(existing, col)
				} else {
					context.AllColumns[strings.ToLower(colName)] = []*database.ColumnDesc{col}
				}
			}
		}
	}

	return context
}

// validateColumnReferences: not used; validation is performed directly in Validate

// validateMemberIdentifier validates a qualified column reference (table.column)
func (v *ColumnValidator) validateMemberIdentifier(member *ast.MemberIdentifier, context *ColumnContext, db *diagnostic.DiagnosticBuilder) {
	if member.ParentIdent == nil || member.ChildIdent == nil {
		return
	}

	tableName := member.ParentIdent.String()
	columnName := member.ChildIdent.String()

	// Resolve alias to actual table name
	if actualTable, ok := context.TableAliases[tableName]; ok {
		tableName = actualTable
	}

	// Check if table exists in context
	cols, ok := context.TableColumns[tableName]
	if !ok {
		// Table not in context, might be a schema.table reference
		return
	}

	// Check if column exists in the table
	found := false
	for _, col := range cols {
		if strings.EqualFold(col.Name, columnName) {
			found = true
			break
		}
	}

	if !found {
        db.AddError(
            member.ChildIdent.Pos(),
            member.ChildIdent.End(),
            diagnostic.CodeColumnNotFound,
            diagnostic.FormatError(diagnostic.CodeColumnNotFound, columnName, tableName),
        )
	}
}

// validateIdentifier validates an unqualified column reference
func (v *ColumnValidator) validateIdentifier(ident *ast.Identifier, context *ColumnContext, db *diagnostic.DiagnosticBuilder) {
	// Skip validation for certain contexts
	if v.shouldSkipIdentifier(ident) {
		return
	}

	columnName := ident.String()

	// Check if this column exists in any of the available tables
	if _, ok := context.AllColumns[columnName]; !ok {
		// Column not found in any table - but we need to be careful here
		// as it might be a function, alias, or other valid identifier
		// Only report if we have tables in context and it looks like a column reference
		if len(context.TableColumns) > 0 && v.looksLikeColumnReference(ident) {
            db.AddError(
                ident.Pos(),
                ident.End(),
                diagnostic.CodeColumnNotFound,
                fmt.Sprintf("Column '%s' not found in any referenced table", columnName),
            )
		}
	}
}

// shouldSkipIdentifier determines if an identifier should skip validation
func (v *ColumnValidator) shouldSkipIdentifier(ident *ast.Identifier) bool {
	// Skip very short identifiers that are likely keywords
	if len(ident.String()) <= 2 {
		return true
	}

	// Skip if it's part of an alias definition
	// This would require more context from the parent node
	// For now, we'll use a simple heuristic

	return false
}

// looksLikeColumnReference determines if an identifier looks like a column reference
func (v *ColumnValidator) looksLikeColumnReference(ident *ast.Identifier) bool {
	// Basic heuristic: if it's not a common SQL keyword, it's likely a column
	name := strings.ToUpper(ident.String())
	commonKeywords := []string{
		"SELECT", "FROM", "WHERE", "JOIN", "LEFT", "RIGHT", "INNER", "OUTER",
		"ON", "AND", "OR", "NOT", "IN", "EXISTS", "BETWEEN", "LIKE",
		"ORDER", "GROUP", "BY", "HAVING", "LIMIT", "OFFSET",
		"INSERT", "UPDATE", "DELETE", "CREATE", "DROP", "ALTER",
		"AS", "ASC", "DESC", "NULL", "IS", "TRUE", "FALSE",
	}

	for _, keyword := range commonKeywords {
		if name == keyword {
			return false
		}
	}

	return true
}

// isStringLiteral determines if an identifier is actually a string literal
func (v *ColumnValidator) isStringLiteral(ident *ast.Identifier) bool {
	if ident.GetToken() == nil {
		return false
	}

	// Check if it's a single-quoted string (standard SQL string literal)
	if ident.GetToken().MatchKind(token.SingleQuotedString) {
		return true
	}

	// Check the raw string representation
	raw := ident.GetToken().String()
	if len(raw) >= 2 {
		// Single quotes are always string literals
		if raw[0] == '\'' {
			return true
		}

		// Double quotes are string literals only in MySQL
		// In other SQL dialects (PostgreSQL, Oracle, etc.), double quotes denote identifiers
		if raw[0] == '"' && v.isMySQLDriver() {
			return true
		}
	}

	return false
}

// isMySQLDriver checks if the current driver is MySQL
func (v *ColumnValidator) isMySQLDriver() bool {
	return v.driver == "mysql" || v.driver == "mysql8" || v.driver == "mysql57" || v.driver == "mysql56"
}

// checkAmbiguousColumns checks for ambiguous column references
// checkAmbiguousColumns handled inline in Validate where context is available

// extractTables builds a table list and alias mapping from parsed query
func (v *ColumnValidator) extractTables(parsed ast.TokenList, aliasMap map[string]string) []*parseutil.TableInfo {
    var toInfos func(n ast.Node) []*parseutil.TableInfo
    toInfos = func(n ast.Node) []*parseutil.TableInfo {
        var out []*parseutil.TableInfo
        switch t := n.(type) {
        case *ast.Identifier:
            out = append(out, &parseutil.TableInfo{Name: t.NoQuoteString()})
        case *ast.MemberIdentifier:
            out = append(out, &parseutil.TableInfo{DatabaseSchema: t.GetParent().String(), Name: t.GetChild().String()})
        case *ast.Aliased:
            // record alias mapping
            if t.AliasedName != nil {
                alias := t.GetAliasedNameIdent().NoQuoteString()
                switch real := t.RealName.(type) {
                case *ast.Identifier:
                    aliasMap[strings.ToLower(alias)] = real.NoQuoteString()
                    out = append(out, &parseutil.TableInfo{Name: real.NoQuoteString(), Alias: alias})
                case *ast.MemberIdentifier:
                    aliasMap[strings.ToLower(alias)] = real.GetChildIdent().NoQuoteString()
                    out = append(out, &parseutil.TableInfo{DatabaseSchema: real.GetParent().String(), Name: real.GetChild().String(), Alias: alias})
                }
            }
        case *ast.IdentifierList:
            for _, id := range t.GetIdentifiers() {
                out = append(out, toInfos(id)...)
            }
        }
        return out
    }

    nodes := []ast.Node{}
    nodes = append(nodes, parseutil.ExtractTableReferences(parsed)...)
    nodes = append(nodes, parseutil.ExtractTableReference(parsed)...)
    nodes = append(nodes, parseutil.ExtractTableFactor(parsed)...)
    infos := []*parseutil.TableInfo{}
    seen := map[string]bool{}
    for _, n := range nodes {
        for _, ti := range toInfos(n) {
            key := strings.ToUpper(ti.DatabaseSchema) + "\t" + strings.ToUpper(ti.Name)
            if !seen[key] {
                infos = append(infos, ti)
                seen[key] = true
            }
        }
    }
    return infos
}

// GetColumnInfo returns information about a column
func (v *ColumnValidator) GetColumnInfo(tableName, columnName string) (*database.ColumnDesc, bool) {
	if v.dbCache == nil {
		return nil, false
	}

	return v.dbCache.Column(tableName, columnName)
}

// GetColumnsForTable returns all columns for a table
func (v *ColumnValidator) GetColumnsForTable(tableName string) ([]*database.ColumnDesc, bool) {
	if v.dbCache == nil {
		return nil, false
	}

	return v.dbCache.ColumnDescs(tableName)
}
