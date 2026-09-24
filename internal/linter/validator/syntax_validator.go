package validator

import (
	"strings"

	"github.com/sqls-server/sqls/ast"
	"github.com/sqls-server/sqls/internal/diagnostic"
	"github.com/sqls-server/sqls/internal/lintconfig"
	"github.com/sqls-server/sqls/parser/parseutil"
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
	seen := make(map[token.Pos]bool)
	var checkProjection func(ast.Node)
	checkProjection = func(node ast.Node) {
		switch n := node.(type) {
		case *ast.IdentifierList:
			for _, ident := range n.GetIdentifiers() {
				checkProjection(ident)
			}
		case *ast.MemberIdentifier:
			checkProjection(n.GetChild())
		case *ast.Identifier:
			if n.IsWildcard() && !seen[n.Pos()] {
				seen[n.Pos()] = true
				db.AddWarning(n.Pos(), n.End(), diagnostic.CodeSelectStar, diagnostic.FormatError(diagnostic.CodeSelectStar))
			}
		}
	}
	// ExtractSelectExpr also visits nested SELECTs. Inspect only projection items,
	// not operators or function arguments such as multiplication and COUNT(*).
	for _, expr := range parseutil.ExtractSelectExpr(parsed) {
		checkProjection(expr)
	}
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
