package handler

import (
	"testing"
	"time"

	"github.com/sqls-server/sqls/internal/config"
	"github.com/sqls-server/sqls/internal/database"
	"github.com/sqls-server/sqls/internal/lsp"
)

func TestLintConfigurationRefresh(t *testing.T) {
	for _, connected := range []bool{false, true} {
		name := "without connection"
		if connected {
			name = "connected"
		}
		t.Run(name, func(t *testing.T) {
			tx := newTestContext()
			tx.diagnostics = make(chan lsp.PublishDiagnosticsParams, 10)
			tx.setup(t)
			defer tx.tearDown()
			if connected {
				tx.server.dbConn = &database.DBConnection{}
			}
			expect := func(count int) {
				t.Helper()
				select {
				case p := <-tx.diagnostics:
					if p.URI != testFileURI || len(p.Diagnostics) != count {
						t.Fatalf("expected %d diagnostics for open document, got %+v", count, p)
					}
				case <-time.After(2 * time.Second):
					t.Fatal("configuration change did not republish diagnostics")
				}
			}
			tx.textDocumentDidOpen(t, testFileURI, "SELECT * FROM customers")
			expect(1)
			cfg := config.NewConfig()
			cfg.Linter.Enabled = false
			tx.addWorkspaceConfig(t, cfg)
			expect(0)
			cfg = config.NewConfig()
			tx.addWorkspaceConfig(t, cfg)
			expect(1)
			cfg = config.NewConfig()
			cfg.Linter.WarnOnSelectStar = false
			tx.addWorkspaceConfig(t, cfg)
			expect(0)
		})
	}
}
