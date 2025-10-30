package validator

import "github.com/sqls-server/sqls/ast"

// walk traverses the AST depth-first
func walk(node ast.Node, fn func(ast.Node)) {
    if node == nil {
        return
    }
    fn(node)
    if tl, ok := node.(ast.TokenList); ok {
        for _, child := range tl.GetTokens() {
            walk(child, fn)
        }
    }
}

// flattenTokens returns all low-level SQL tokens within a node in-order
func flattenTokens(node ast.Node) []*ast.SQLToken {
    toks := []*ast.SQLToken{}
    walk(node, func(n ast.Node) {
        if tok, ok := n.(ast.Token); ok {
            toks = append(toks, tok.GetToken())
        }
    })
    return toks
}

