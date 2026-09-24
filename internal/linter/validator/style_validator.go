package validator

import (
    "strings"

    "github.com/sqls-server/sqls/ast"
    "github.com/sqls-server/sqls/dialect"
    "github.com/sqls-server/sqls/internal/diagnostic"
    "github.com/sqls-server/sqls/internal/lintconfig"
    "github.com/sqls-server/sqls/parser/parseutil"
    "github.com/sqls-server/sqls/token"
)

// StyleValidator validates SQL style conventions
type StyleValidator struct {
    config  *lintconfig.Config
    dialect dialect.Dialect
}

// NewStyleValidator creates a new style validator
func NewStyleValidator(config *lintconfig.Config, d dialect.Dialect) *StyleValidator {
    if d == nil {
        d = &dialect.GenericSQLDialect{}
    }
    return &StyleValidator{
        config:  config,
        dialect: d,
    }
}

// Validate performs style validation
func (v *StyleValidator) Validate(parsed ast.TokenList, db *diagnostic.DiagnosticBuilder) {
    // Reserved keyword case
    if v.config.IsRuleEnabled(v.config.CheckReservedWordCase) {
        v.checkReservedWordCase(parsed, db)
    }
    // Restore missing semicolon check
    if v.config.IsRuleEnabled(v.config.CheckMissingSemicolon) {
        v.checkMissingSemicolon(parsed, db)
    }
}

// checkReservedWordCase checks if reserved words follow the configured case convention
func (v *StyleValidator) checkReservedWordCase(parsed ast.TokenList, db *diagnostic.DiagnosticBuilder) {
    preferUpper := strings.ToLower(v.config.PreferredKeywordCase) == "upper"
    sev := lintconfig.GetDiagnosticSeverity(v.config.CheckReservedWordCase)

    toks := flattenTokens(parsed)
    for _, t := range toks {
        if t.Kind != token.SQLKeyword {
            continue
        }
        w, ok := t.Value.(*token.SQLWord)
        if !ok {
            continue
        }
        val := w.String()
        isUpper := val == strings.ToUpper(val)
        isLower := val == strings.ToLower(val)
        if preferUpper && !isUpper && isLower {
            v.emitCaseDiagnostic(t.From, t.To, "uppercase", sev, db)
        }
        if !preferUpper && !isLower && isUpper {
            v.emitCaseDiagnostic(t.From, t.To, "lowercase", sev, db)
        }
    }
}

// addCaseDiagnostic adds a case-related diagnostic
func (v *StyleValidator) emitCaseDiagnostic(from, to token.Pos, expectedCase string, severity diagnostic.DiagnosticSeverity, db *diagnostic.DiagnosticBuilder) {
    // We don’t have the exact keyword text here in all cases; show generic message
    message := diagnostic.FormatError(diagnostic.CodeReservedWordCase, "keyword", expectedCase)
    switch severity {
    case diagnostic.SeverityError:
        db.AddError(from, to, diagnostic.CodeReservedWordCase, message)
    case diagnostic.SeverityWarning:
        db.AddWarning(from, to, diagnostic.CodeReservedWordCase, message)
    case diagnostic.SeverityInfo:
        db.AddInfo(from, to, diagnostic.CodeReservedWordCase, message)
    case diagnostic.SeverityHint:
        db.AddHint(from, to, diagnostic.CodeReservedWordCase, message)
    }
}

// isReservedKeyword checks if a word is a reserved keyword
func (v *StyleValidator) isReservedKeyword(word string) bool {
    upperWord := strings.ToUpper(word)

	// Common SQL keywords
	reservedKeywords := []string{
		"SELECT", "FROM", "WHERE", "JOIN", "LEFT", "RIGHT", "INNER", "OUTER",
		"ON", "AND", "OR", "NOT", "IN", "EXISTS", "BETWEEN", "LIKE",
		"ORDER", "GROUP", "BY", "HAVING", "LIMIT", "OFFSET",
		"INSERT", "INTO", "VALUES", "UPDATE", "SET", "DELETE",
		"CREATE", "DROP", "ALTER", "TABLE", "DATABASE", "SCHEMA",
		"INDEX", "VIEW", "TRIGGER", "PROCEDURE", "FUNCTION",
		"AS", "ASC", "DESC", "NULL", "IS", "TRUE", "FALSE",
		"DISTINCT", "ALL", "ANY", "SOME", "UNION", "INTERSECT", "EXCEPT",
		"CASE", "WHEN", "THEN", "ELSE", "END",
		"PRIMARY", "KEY", "FOREIGN", "REFERENCES", "UNIQUE", "CHECK",
		"DEFAULT", "AUTO_INCREMENT", "CONSTRAINT",
	}

	for _, keyword := range reservedKeywords {
		if upperWord == keyword {
			return true
		}
	}

    return false
}

// checkMissingSemicolon checks for missing semicolons at end of statements
func (v *StyleValidator) checkMissingSemicolon(parsed ast.TokenList, db *diagnostic.DiagnosticBuilder) {
    severity := lintconfig.GetDiagnosticSeverity(v.config.CheckMissingSemicolon)
    walk(parsed, func(n ast.Node) {
        stmt, ok := n.(*ast.Statement)
        if !ok {
            return
        }
        toks := flattenTokens(stmt)
        if len(toks) == 0 {
            return
        }
        // Find last non-whitespace/comment token
        var last *ast.SQLToken
        for i := len(toks) - 1; i >= 0; i-- {
            k := toks[i].Kind
            if k == token.Whitespace || k == token.Comment || k == token.MultilineComment {
                continue
            }
            last = toks[i]
            break
        }
        if last == nil {
            return
        }
        if last.Kind != token.Semicolon {
            end := stmt.End()
            msg := diagnostic.FormatError(diagnostic.CodeMissingSemicolon)
            switch severity {
            case diagnostic.SeverityError:
                db.AddError(end, end, diagnostic.CodeMissingSemicolon, msg)
            case diagnostic.SeverityWarning:
                db.AddWarning(end, end, diagnostic.CodeMissingSemicolon, msg)
            case diagnostic.SeverityInfo:
                db.AddInfo(end, end, diagnostic.CodeMissingSemicolon, msg)
            case diagnostic.SeverityHint:
                db.AddHint(end, end, diagnostic.CodeMissingSemicolon, msg)
            }
        }
    })
}

// checkStatementSemicolon checks if a statement ends with a semicolon
// CheckUnusedAliases checks for defined but unused aliases
func CheckUnusedAliases(parsed ast.TokenList, db *diagnostic.DiagnosticBuilder, config *lintconfig.Config) {
    if !config.WarnOnUnusedAlias {
        return
    }
    // Collect alias definitions (excluding subqueries)
    defs := map[string]*ast.Identifier{}
    for _, node := range parseutil.ExtractAliasedIdentifier(parsed) {
        if aliased, ok := node.(*ast.Aliased); ok {
            ident := aliased.GetAliasedNameIdent()
            if ident != nil {
                defs[strings.ToLower(ident.NoQuoteString())] = ident
            }
        }
    }
    if len(defs) == 0 {
        return
    }
    // Collect usages: parent part of member identifiers
    used := map[string]bool{}
    walk(parsed, func(n ast.Node) {
        if m, ok := n.(*ast.MemberIdentifier); ok {
            if m.ParentIdent != nil {
                name := strings.ToLower(m.ParentIdent.NoQuoteString())
                used[name] = true
            }
        }
    })
    for name, ident := range defs {
        if !used[name] {
            db.AddWarning(
                ident.Pos(),
                ident.End(),
                diagnostic.CodeUnusedAlias,
                diagnostic.FormatError(diagnostic.CodeUnusedAlias, ident.NoQuoteString()),
            )
        }
    }
}

// helpers are in util.go
