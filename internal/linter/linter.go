package linter

import (
    "github.com/sqls-server/sqls/ast"
    "github.com/sqls-server/sqls/dialect"
    "github.com/sqls-server/sqls/internal/database"
    "github.com/sqls-server/sqls/internal/diagnostic"
    "github.com/sqls-server/sqls/internal/lintconfig"
    "github.com/sqls-server/sqls/internal/linter/validator"
    "github.com/sqls-server/sqls/parser"
    "github.com/sqls-server/sqls/token"
)

// Linter is the main linter coordinator
type Linter struct {
    config           *lintconfig.Config
    dbCache          *database.DBCache
    dialect          dialect.Dialect
    driver           string // Database driver string (e.g., "mysql", "postgresql")
    syntaxValidator  *validator.SyntaxValidator
    tableValidator   *validator.TableValidator
    columnValidator  *validator.ColumnValidator
    styleValidator   *validator.StyleValidator
}

// New creates a new linter instance
func New(config *lintconfig.Config, dbCache *database.DBCache, dialect dialect.Dialect, driver string) *Linter {
    if config == nil {
        config = lintconfig.DefaultConfig()
    }

    return &Linter{
        config:          config,
        dbCache:         dbCache,
        dialect:         dialect,
        driver:          driver,
        syntaxValidator: validator.NewSyntaxValidator(config),
        tableValidator:  validator.NewTableValidator(config, dbCache),
        columnValidator: validator.NewColumnValidator(config, dbCache, driver),
        styleValidator:  validator.NewStyleValidator(config, dialect),
    }
}

// Lint performs linting on SQL text and returns diagnostics
func (l *Linter) Lint(text string) ([]diagnostic.Diagnostic, error) {
    if !l.config.Enabled {
        return nil, nil
    }

    db := diagnostic.NewDiagnosticBuilder()

    // Parse the SQL
    parsed, err := parser.Parse(text)
    if err != nil {
        // If parsing fails completely, report a syntax error
        db.AddError(
            token.Pos{Line: 0, Col: 0},
            token.Pos{Line: 0, Col: 0},
            diagnostic.CodeSyntaxError,
            "Failed to parse SQL: "+err.Error(),
        )
        return l.limitDiagnostics(db.Build()), nil
    }

    // Run validators in order
    l.runValidators(text, parsed, db)

    diagnostics := db.Build()
    return l.limitDiagnostics(diagnostics), nil
}

// runValidators runs all enabled validators
func (l *Linter) runValidators(text string, parsed ast.TokenList, db *diagnostic.DiagnosticBuilder) {
    // 1. Syntax validation
    if l.config.CheckSyntax {
        l.syntaxValidator.Validate(parsed, db)
    }

	// 2. Table validation
	if l.config.CheckTableReferences {
		l.tableValidator.Validate(text, db)
	}

	// 3. Column validation
    if l.config.CheckColumnReferences {
        l.columnValidator.Validate(text, db)
    }

	// 4. Style validation
	l.styleValidator.Validate(parsed, db)

    // 5. Additional checks
    if l.config.WarnOnSelectStar {
        validator.CheckSelectStar(parsed, db, l.config)
    }

    if l.config.WarnOnUnusedAlias {
        validator.CheckUnusedAliases(parsed, db, l.config)
    }

    if l.config.WarnOnImplicitJoin {
        l.tableValidator.CheckImplicitJoins(text, db)
    }
}

// limitDiagnostics limits the number of diagnostics returned
func (l *Linter) limitDiagnostics(diagnostics []diagnostic.Diagnostic) []diagnostic.Diagnostic {
    if l.config.MaxDiagnostics > 0 && len(diagnostics) > l.config.MaxDiagnostics {
        return diagnostics[:l.config.MaxDiagnostics]
    }
    return diagnostics
}

// UpdateConfig updates the linter configuration
func (l *Linter) UpdateConfig(config *lintconfig.Config) {
    l.config = config
    l.syntaxValidator = validator.NewSyntaxValidator(config)
    l.tableValidator = validator.NewTableValidator(config, l.dbCache)
    l.columnValidator = validator.NewColumnValidator(config, l.dbCache, l.driver)
    l.styleValidator = validator.NewStyleValidator(config, l.dialect)
}

// UpdateDBCache updates the database cache
func (l *Linter) UpdateDBCache(dbCache *database.DBCache) {
    l.dbCache = dbCache
    l.tableValidator = validator.NewTableValidator(l.config, dbCache)
    l.columnValidator = validator.NewColumnValidator(l.config, dbCache, l.driver)
}

// UpdateDialect updates the SQL dialect
func (l *Linter) UpdateDialect(dialect dialect.Dialect) {
    l.dialect = dialect
    l.styleValidator = validator.NewStyleValidator(l.config, dialect)
}

// GetConfig returns the current configuration
func (l *Linter) GetConfig() *lintconfig.Config {
    return l.config
}

// IsEnabled returns whether the linter is enabled
func (l *Linter) IsEnabled() bool {
    return l.config.Enabled
}
