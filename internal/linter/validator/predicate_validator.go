package validator

import (
	"fmt"
	"slices"
	"strings"

	"github.com/sqls-server/sqls/ast"
	"github.com/sqls-server/sqls/internal/database"
	"github.com/sqls-server/sqls/internal/diagnostic"
	"github.com/sqls-server/sqls/parser/parseutil"
	"github.com/sqls-server/sqls/token"
)

type predicateClause struct {
	keyword *ast.SQLToken
	expr    []*ast.SQLToken
}

func predicateKeyword(tok *ast.SQLToken) string {
	if word, ok := tok.Value.(*token.SQLWord); tok.Kind == token.SQLKeyword && ok && word.QuoteStyle == 0 {
		return word.Keyword
	}
	return ""
}

// The editor parser deliberately accepts unfinished SQL. Collect predicates
// from its tokens, preserving nested query boundaries and diagnostic ranges.
func predicateClauses(parsed ast.TokenList) []predicateClause {
	tokens := []*ast.SQLToken{}
	for _, tok := range flattenTokens(parsed) {
		if tok.Kind != token.Whitespace && tok.Kind != token.Comment && tok.Kind != token.MultilineComment {
			tokens = append(tokens, tok)
		}
	}
	var clauses []predicateClause
	for i, tok := range tokens {
		if kw := predicateKeyword(tok); kw != "WHERE" && kw != "HAVING" {
			continue
		}
		depth, end := 0, i+1
		for ; end < len(tokens); end++ {
			next := tokens[end]
			if depth == 0 {
				if next.Kind == token.Semicolon || next.Kind == token.RParen {
					break
				}
				switch predicateKeyword(next) {
				case "GROUP", "ORDER", "HAVING", "LIMIT", "OFFSET", "UNION", "INTERSECT", "EXCEPT", "RETURNING", "WINDOW", "FETCH", "FOR", "QUALIFY":
					goto clauseEnd
				}
			}
			if next.Kind == token.LParen {
				depth++
			}
			if next.Kind == token.RParen {
				depth--
			}
		}
	clauseEnd:
		clauses = append(clauses, predicateClause{keyword: tok, expr: tokens[i+1 : end]})
	}
	return clauses
}

func unwrapPredicate(expr []*ast.SQLToken) []*ast.SQLToken {
	for len(expr) >= 2 && expr[0].Kind == token.LParen && expr[len(expr)-1].Kind == token.RParen {
		depth, encloses := 0, true
		for i, tok := range expr {
			if tok.Kind == token.LParen {
				depth++
			}
			if tok.Kind == token.RParen {
				depth--
			}
			if depth == 0 && i != len(expr)-1 {
				encloses = false
				break
			}
		}
		if !encloses {
			break
		}
		expr = expr[1 : len(expr)-1]
	}
	return expr
}

// Visit the operands of logical operators without splitting BETWEEN's AND or
// the branches inside CASE. Complex scalar expressions retain their tokens.
func predicateOperands(expr []*ast.SQLToken, anchor *ast.SQLToken, visit func([]*ast.SQLToken, *ast.SQLToken)) {
	expr = unwrapPredicate(expr)
	depth, cases, between := 0, 0, false
	for i, tok := range expr {
		if tok.Kind == token.LParen {
			depth++
		}
		if tok.Kind == token.RParen {
			depth--
		}
		if depth != 0 {
			continue
		}
		switch predicateKeyword(tok) {
		case "CASE":
			cases++
		case "END":
			if cases > 0 {
				cases--
			}
		}
		if cases != 0 {
			continue
		}
		switch predicateKeyword(tok) {
		case "BETWEEN":
			between = true
		case "AND":
			if between {
				between = false
				continue
			}
			predicateOperands(expr[:i], anchor, visit)
			predicateOperands(expr[i+1:], tok, visit)
			return
		case "OR":
			predicateOperands(expr[:i], anchor, visit)
			predicateOperands(expr[i+1:], tok, visit)
			return
		}
	}
	if len(expr) > 0 && predicateKeyword(expr[0]) == "NOT" {
		predicateOperands(expr[1:], expr[0], visit)
		return
	}
	visit(expr, anchor)
}

func checkPredicateSyntax(parsed ast.TokenList, db *diagnostic.DiagnosticBuilder) {
	for _, clause := range predicateClauses(parsed) {
		predicateOperands(clause.expr, clause.keyword, func(expr []*ast.SQLToken, anchor *ast.SQLToken) {
			if len(expr) == 0 {
				db.AddError(anchor.From, anchor.To, diagnostic.CodeSyntaxError,
					fmt.Sprintf("Expected an expression after %s", predicateKeyword(anchor)))
				return
			}
			last := expr[len(expr)-1]
			missingOperand := slices.Contains([]token.Kind{
				token.Eq, token.Neq, token.Lt, token.Gt, token.LtEq, token.GtEq,
				token.Plus, token.Minus, token.Mult, token.Div, token.Mod, token.Caret, token.Period, token.Comma,
			}, last.Kind)
			switch predicateKeyword(last) {
			case "IS", "NOT", "IN", "LIKE", "ILIKE", "BETWEEN", "EXISTS", "REGEXP", "GLOB", "MATCH":
				missingOperand = true
			}
			if missingOperand {
				db.AddError(last.From, last.To, diagnostic.CodeSyntaxError, "Incomplete predicate: expected an expression after this operator")
			}
		})
	}
}

// CheckPredicateTypes diagnoses known non-boolean operands in PostgreSQL.
// SQLite and MySQL intentionally accept numeric truth values. Unknown types,
// functions and user-defined domains are left to the database's type system.
func CheckPredicateTypes(parsed ast.TokenList, cache *database.DBCache, driver string, db *diagnostic.DiagnosticBuilder) {
	if driver != "postgresql" {
		return
	}
	for _, clause := range predicateClauses(parsed) {
		predicateOperands(clause.expr, clause.keyword, func(expr []*ast.SQLToken, anchor *ast.SQLToken) {
			typ := predicateType(parsed, expr, cache)
			if !knownNonBooleanType(typ) {
				return
			}
			name := ""
			for _, tok := range expr {
				name += fmt.Sprint(tok.Value)
			}
			db.AddError(expr[0].From, expr[len(expr)-1].To, diagnostic.CodeTypeMismatch,
				fmt.Sprintf("%s requires a boolean expression, but '%s' has type %s; use an explicit comparison", predicateKeyword(clause.keyword), name, typ))
		})
	}
}

func predicateType(parsed ast.TokenList, expr []*ast.SQLToken, cache *database.DBCache) string {
	if len(expr) == 1 && expr[0].Kind == token.Number {
		return "numeric"
	}
	if cache == nil || (len(expr) != 1 && len(expr) != 3) {
		return ""
	}
	first, ok := expr[0].Value.(*token.SQLWord)
	if !ok || expr[0].Kind != token.SQLKeyword {
		return ""
	}
	qualifier, column := "", first.Value
	if len(expr) == 3 {
		last, ok := expr[2].Value.(*token.SQLWord)
		if !ok || expr[1].Kind != token.Period || expr[2].Kind != token.SQLKeyword {
			return ""
		}
		qualifier, column = first.Value, last.Value
	}
	// Probe within the token: the parser's cursor walker includes token ends.
	pos := expr[0].From
	pos.Col++
	tables, err := parseutil.ExtractTable(parsed, pos)
	if err != nil {
		return ""
	}
	var matches []*database.ColumnDesc
	for _, table := range tables {
		if qualifier != "" && !strings.EqualFold(qualifier, table.Alias) && !strings.EqualFold(qualifier, table.Name) {
			continue
		}
		var cols []*database.ColumnDesc
		if table.DatabaseSchema != "" {
			cols, _ = cache.ColumnDatabase(table.DatabaseSchema, table.Name)
		} else {
			cols, _ = cache.ColumnDescs(table.Name)
		}
		for _, col := range cols {
			if strings.EqualFold(col.Name, column) {
				matches = append(matches, col)
			}
		}
	}
	if len(matches) == 1 {
		return matches[0].Type
	}
	return ""
}

func knownNonBooleanType(typ string) bool {
	typ = strings.ToLower(strings.TrimSpace(typ))
	if i := strings.IndexByte(typ, '('); i >= 0 {
		typ = strings.TrimSpace(typ[:i])
	}
	switch typ {
	case "smallint", "integer", "bigint", "int", "int2", "int4", "int8", "numeric", "decimal", "real", "double precision", "float4", "float8",
		"text", "varchar", "character varying", "char", "character", "date", "timestamp", "timestamp without time zone", "timestamp with time zone", "time", "interval", "uuid", "json", "jsonb", "bytea":
		return true
	}
	return false
}
