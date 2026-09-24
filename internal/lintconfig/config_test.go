package lintconfig

import (
	"testing"

	"github.com/sqls-server/sqls/internal/diagnostic"
)

func TestRuleSeverity(t *testing.T) {
	for _, tc := range []struct {
		rule RuleSeverity
		want diagnostic.DiagnosticSeverity
	}{
		{RuleSeverityOff, 0}, {RuleSeverityError, diagnostic.SeverityError},
		{RuleSeverityWarning, diagnostic.SeverityWarning}, {RuleSeverityInfo, diagnostic.SeverityInfo},
		{RuleSeverityHint, diagnostic.SeverityHint},
	} {
		t.Run(string(tc.rule), func(t *testing.T) {
			if got := GetDiagnosticSeverity(tc.rule); got != tc.want {
				t.Fatalf("want %d, got %d", tc.want, got)
			}
			if got := DefaultConfig().IsRuleEnabled(tc.rule); got != (tc.rule != RuleSeverityOff) {
				t.Fatalf("unexpected enabled state: %v", got)
			}
		})
	}
}
