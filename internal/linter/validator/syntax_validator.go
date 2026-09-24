package validator

import (
    "strings"

    "github.com/sqls-server/sqls/ast"
    "github.com/sqls-server/sqls/internal/diagnostic"
    "github.com/sqls-server/sqls/internal/lintconfig"
    "github.com/sqls-server/sqls/token"
)

// SyntaxValidator validates SQL syntax
type SyntaxValidator struct {
    config *lintconfig.Config
}

// NewSyntaxValidator creates a new syntax validator
func NewSyntaxValidator(config *lintconfig.Config) *SyntaxValidator {
    return &SyntaxValidator{config: config}
}

// Validate performs core syntax validations on the AST
func (v *SyntaxValidator) Validate(parsed ast.TokenList, db *diagnostic.DiagnosticBuilder) {
    // Minimal: detect NULL comparisons with = or !=
    v.checkNullComparisons(parsed, db)
}

// CheckSelectStar warns when SELECT * is used
func CheckSelectStar(parsed ast.TokenList, db *diagnostic.DiagnosticBuilder, config *lintconfig.Config) {
    if !config.WarnOnSelectStar {
        return
    }
    // For each statement, look for SELECT followed by '*'
    walk(parsed, func(n ast.Node) {
        stmt, ok := n.(*ast.Statement)
        if !ok {
            return
        }
        toks := flattenTokens(stmt)
        sawSelect := false
        for i := 0; i < len(toks); i++ {
            t := toks[i]
            if t.Kind == token.SQLKeyword {
                if w, ok := t.Value.(*token.SQLWord); ok && strings.EqualFold(w.Keyword, "SELECT") {
                    sawSelect = true
                    continue
                }
            }
            if sawSelect {
                if t.Kind == token.Mult {
                    // Found SELECT *
                    db.AddWarning(t.From, t.To, diagnostic.CodeSelectStar, diagnostic.FormatError(diagnostic.CodeSelectStar))
                    break
                }
                // Stop checking at FROM
                if t.Kind == token.SQLKeyword {
                    if w, ok := t.Value.(*token.SQLWord); ok && strings.EqualFold(w.Keyword, "FROM") {
                        break
                    }
                }
            }
        }
    })
}

// checkNullComparisons checks for incorrect NULL comparisons
func (v *SyntaxValidator) checkNullComparisons(parsed ast.TokenList, db *diagnostic.DiagnosticBuilder) {
    if !v.config.WarnOnNullComparison {
        return
    }
    walk(parsed, func(n ast.Node) {
        comp, ok := n.(*ast.Comparison)
        if !ok {
            return
        }
        // Scan tokens within the comparison for NULL and operators
        toks := flattenTokens(comp)
        hasNull := false
        var opFrom, opTo token.Pos
        for _, t := range toks {
            if t.Kind == token.SQLKeyword {
                if w, ok := t.Value.(*token.SQLWord); ok && strings.EqualFold(w.Keyword, "NULL") {
                    hasNull = true
                }
            }
            if t.Kind == token.Eq || t.Kind == token.Neq {
                opFrom, opTo = t.From, t.To
            }
        }
        if hasNull && (opFrom != (token.Pos{})) {
            db.AddWarning(opFrom, opTo, diagnostic.CodeNullComparison, diagnostic.FormatError(diagnostic.CodeNullComparison))
        }
    })
}

// helpers moved to util.go
