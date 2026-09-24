package handler

import (
    "context"
    "log"

    "github.com/sourcegraph/jsonrpc2"
    "github.com/sqls-server/sqls/dialect"
    "github.com/sqls-server/sqls/internal/database"
    "github.com/sqls-server/sqls/internal/diagnostic"
    "github.com/sqls-server/sqls/internal/lintconfig"
    "github.com/sqls-server/sqls/internal/linter"
    "github.com/sqls-server/sqls/internal/lsp"
)

// publishDiagnostics sends diagnostics to the client
func (s *Server) publishDiagnostics(ctx context.Context, conn *jsonrpc2.Conn, uri string, diagnostics []diagnostic.Diagnostic) error {
    // convert to LSP diagnostics
    lspDiags := make([]lsp.Diagnostic, 0, len(diagnostics))
    for _, d := range diagnostics {
        lspDiags = append(lspDiags, lsp.Diagnostic{
            Range: lsp.Range{
                Start: lsp.Position{Line: d.Range.Start.Line, Character: d.Range.Start.Character},
                End:   lsp.Position{Line: d.Range.End.Line, Character: d.Range.End.Character},
            },
            Severity: int(d.Severity),
            Code:     string(d.Code),
            Source:   d.Source,
            Message:  d.Message,
        })
    }
    params := lsp.PublishDiagnosticsParams{
        URI:         uri,
        Diagnostics: lspDiags,
    }

	return conn.Notify(ctx, "textDocument/publishDiagnostics", params)
}

// lintDocument performs linting on a document and publishes diagnostics
func (s *Server) lintDocument(ctx context.Context, conn *jsonrpc2.Conn, uri string) error {
	// Get the file content
	file, ok := s.files[uri]
	if !ok {
		return nil
	}

	// Ensure linter is initialized
	if s.linter == nil {
		s.initializeLinter()
	}

    // Update linter with current database cache
    if s.worker != nil {
        s.linter.UpdateDBCache(s.worker.Cache())
    }

	// Perform linting
	diagnostics, err := s.linter.Lint(file.Text)
	if err != nil {
		log.Printf("linting error: %v", err)
		return nil // Don't fail the entire operation on lint error
	}

    // Publish diagnostics
    return s.publishDiagnostics(ctx, conn, uri, diagnostics)
}

// initializeLinter initializes the linter with the current configuration
func (s *Server) initializeLinter() {
	cfg := s.getConfig()
    var linterCfg *lintconfig.Config
    if cfg != nil && cfg.Linter != nil {
        linterCfg = cfg.Linter
    } else {
        linterCfg = lintconfig.DefaultConfig()
    }

    // Use a generic SQL dialect for linting
    var dialectObj dialect.Dialect = &dialect.GenericSQLDialect{}

    // Get database cache
    var dbCache *database.DBCache
    if s.worker != nil {
        dbCache = s.worker.Cache()
    }

    // Get database driver
    var driver string
    if s.curDBCfg != nil {
        driver = string(s.curDBCfg.Driver)
    }

	s.linter = linter.New(linterCfg, dbCache, dialectObj, driver)
}

// clearDiagnostics clears diagnostics for a document
func (s *Server) clearDiagnostics(ctx context.Context, conn *jsonrpc2.Conn, uri string) error {
    return s.publishDiagnostics(ctx, conn, uri, []diagnostic.Diagnostic{})
}

// updateLinterConfig updates the linter configuration
func (s *Server) updateLinterConfig() {
    if s.linter != nil {
        cfg := s.getConfig()
        if cfg != nil && cfg.Linter != nil {
            s.linter.UpdateConfig(cfg.Linter)
        }
    }
}
