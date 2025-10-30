package diagnostic

import (
    "fmt"

    "github.com/sqls-server/sqls/token"
)

// DiagnosticSeverity represents the severity level of a diagnostic
type DiagnosticSeverity int

const (
    SeverityError   DiagnosticSeverity = 1
    SeverityWarning DiagnosticSeverity = 2
    SeverityInfo    DiagnosticSeverity = 3
    SeverityHint    DiagnosticSeverity = 4
)

// DiagnosticCode represents the type of diagnostic
type DiagnosticCode string

const (
    // Syntax errors
    CodeSyntaxError         DiagnosticCode = "syntax-error"
    CodeUnclosedString      DiagnosticCode = "unclosed-string"
    CodeUnclosedParenthesis DiagnosticCode = "unclosed-parenthesis"
    CodeInvalidToken        DiagnosticCode = "invalid-token"
    CodeMissingClause       DiagnosticCode = "missing-clause"

    // Semantic errors
    CodeInvalidTable    DiagnosticCode = "invalid-table"
    CodeInvalidColumn   DiagnosticCode = "invalid-column"
    CodeAmbiguousColumn DiagnosticCode = "ambiguous-column"
    CodeInvalidSchema   DiagnosticCode = "invalid-schema"
    CodeInvalidDatabase DiagnosticCode = "invalid-database"
    CodeTableNotFound   DiagnosticCode = "table-not-found"
    CodeColumnNotFound  DiagnosticCode = "column-not-found"
    CodeTypeMismatch    DiagnosticCode = "type-mismatch"

    // Semantic warnings
    CodeUnusedAlias       DiagnosticCode = "unused-alias"
    CodeSelectStar        DiagnosticCode = "select-star"
    CodeMissingTableAlias DiagnosticCode = "missing-table-alias"
    CodeNullComparison    DiagnosticCode = "null-comparison"
    CodeCaseMismatch      DiagnosticCode = "case-mismatch"
    CodeImplicitJoin      DiagnosticCode = "implicit-join"

    // Style hints
    CodeReservedWordCase   DiagnosticCode = "reserved-word-case"
    CodeInconsistentNaming DiagnosticCode = "inconsistent-naming"
    CodeMissingSemicolon   DiagnosticCode = "missing-semicolon"
    CodeTrailingWhitespace DiagnosticCode = "trailing-whitespace"
)

// Range is an LSP-like range but independent of LSP package
type Range struct {
    Start Position `json:"start"`
    End   Position `json:"end"`
}

type Position struct {
    Line      int `json:"line"`
    Character int `json:"character"`
}

// Diagnostic represents a linting diagnostic
type Diagnostic struct {
    Range    Range
    Severity DiagnosticSeverity
    Code     DiagnosticCode
    Source   string
    Message  string
}

// DiagnosticBuilder helps construct diagnostics
type DiagnosticBuilder struct {
    diagnostics []Diagnostic
}

// NewDiagnosticBuilder creates a new diagnostic builder
func NewDiagnosticBuilder() *DiagnosticBuilder {
    return &DiagnosticBuilder{diagnostics: make([]Diagnostic, 0)}
}

// Add adds a diagnostic
func (db *DiagnosticBuilder) Add(d Diagnostic) {
    db.diagnostics = append(db.diagnostics, d)
}

// AddError adds an error diagnostic
func (db *DiagnosticBuilder) AddError(start, end token.Pos, code DiagnosticCode, message string) {
    db.Add(Diagnostic{
        Range:    posToRange(start, end),
        Severity: SeverityError,
        Code:     code,
        Source:   "sqls",
        Message:  message,
    })
}

// AddWarning adds a warning diagnostic
func (db *DiagnosticBuilder) AddWarning(start, end token.Pos, code DiagnosticCode, message string) {
    db.Add(Diagnostic{
        Range:    posToRange(start, end),
        Severity: SeverityWarning,
        Code:     code,
        Source:   "sqls",
        Message:  message,
    })
}

// AddInfo adds an info diagnostic
func (db *DiagnosticBuilder) AddInfo(start, end token.Pos, code DiagnosticCode, message string) {
    db.Add(Diagnostic{
        Range:    posToRange(start, end),
        Severity: SeverityInfo,
        Code:     code,
        Source:   "sqls",
        Message:  message,
    })
}

// AddHint adds a hint diagnostic
func (db *DiagnosticBuilder) AddHint(start, end token.Pos, code DiagnosticCode, message string) {
    db.Add(Diagnostic{
        Range:    posToRange(start, end),
        Severity: SeverityHint,
        Code:     code,
        Source:   "sqls",
        Message:  message,
    })
}

// Build returns all diagnostics
func (db *DiagnosticBuilder) Build() []Diagnostic {
    return db.diagnostics
}

// posToRange converts token positions to our Range type
func posToRange(start, end token.Pos) Range {
    return Range{
        Start: Position{Line: start.Line - 1, Character: start.Col - 1},
        End:   Position{Line: end.Line - 1, Character: end.Col - 1},
    }
}

// CreateDiagnostic is a helper to create a diagnostic quickly
func CreateDiagnostic(start, end token.Pos, severity DiagnosticSeverity, code DiagnosticCode, message string) Diagnostic {
    return Diagnostic{
        Range:    posToRange(start, end),
        Severity: severity,
        Code:     code,
        Source:   "sqls",
        Message:  message,
    }
}

// FormatError creates a formatted error message
func FormatError(code DiagnosticCode, args ...interface{}) string {
    messages := map[DiagnosticCode]string{
        CodeTableNotFound:       "Table '%s' not found",
        CodeColumnNotFound:      "Column '%s' not found in table '%s'",
        CodeAmbiguousColumn:     "Column '%s' is ambiguous, found in tables: %s",
        CodeInvalidSchema:       "Schema '%s' does not exist",
        CodeInvalidDatabase:     "Database '%s' does not exist",
        CodeUnclosedString:      "Unclosed string literal",
        CodeUnclosedParenthesis: "Unclosed parenthesis",
        CodeSelectStar:          "Use of SELECT * is discouraged, specify columns explicitly",
        CodeNullComparison:      "Comparing with NULL using '=' or '!=', use 'IS NULL' or 'IS NOT NULL'",
        CodeUnusedAlias:         "Alias '%s' is defined but never used",
        CodeReservedWordCase:    "Reserved word '%s' should be %s",
        CodeMissingSemicolon:    "Missing semicolon at end of statement",
    }

    if template, ok := messages[code]; ok {
        return fmt.Sprintf(template, args...)
    }
    return fmt.Sprintf("%s: %v", code, args)
}

