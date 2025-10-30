package lintconfig

import "github.com/sqls-server/sqls/internal/diagnostic"

// RuleSeverity represents how a rule should be treated
type RuleSeverity string

const (
    RuleSeverityError   RuleSeverity = "error"
    RuleSeverityWarning RuleSeverity = "warning"
    RuleSeverityInfo    RuleSeverity = "info"
    RuleSeverityHint    RuleSeverity = "hint"
    RuleSeverityOff     RuleSeverity = "off"
)

// Config represents the linter configuration
type Config struct {
    // Enable/disable the linter entirely
    Enabled bool `yaml:"enabled" json:"enabled"`

    // Syntax rules
    CheckSyntax bool `yaml:"checkSyntax" json:"checkSyntax"`

    // Semantic rules
    CheckTableReferences  bool `yaml:"checkTableReferences" json:"checkTableReferences"`
    CheckColumnReferences bool `yaml:"checkColumnReferences" json:"checkColumnReferences"`
    CheckSchemaReferences bool `yaml:"checkSchemaReferences" json:"checkSchemaReferences"`

    // Semantic warnings
    WarnOnSelectStar      bool `yaml:"warnOnSelectStar" json:"warnOnSelectStar"`
    WarnOnNullComparison  bool `yaml:"warnOnNullComparison" json:"warnOnNullComparison"`
    WarnOnUnusedAlias     bool `yaml:"warnOnUnusedAlias" json:"warnOnUnusedAlias"`
    WarnOnImplicitJoin    bool `yaml:"warnOnImplicitJoin" json:"warnOnImplicitJoin"`
    WarnOnAmbiguousColumn bool `yaml:"warnOnAmbiguousColumn" json:"warnOnAmbiguousColumn"`

    // Style rules
    CheckReservedWordCase RuleSeverity `yaml:"checkReservedWordCase" json:"checkReservedWordCase"`
    PreferredKeywordCase  string       `yaml:"preferredKeywordCase" json:"preferredKeywordCase"` // "upper" or "lower"
    CheckMissingSemicolon RuleSeverity `yaml:"checkMissingSemicolon" json:"checkMissingSemicolon"`

    // Advanced options
    MaxDiagnostics int  `yaml:"maxDiagnostics" json:"maxDiagnostics"`
    LintOnChange   bool `yaml:"lintOnChange" json:"lintOnChange"`
    LintOnSave     bool `yaml:"lintOnSave" json:"lintOnSave"`
    DebugMode      bool `yaml:"debugMode" json:"debugMode"`
}

// DefaultConfig returns the default linter configuration
func DefaultConfig() *Config {
    return &Config{
        Enabled: true,

        // Syntax checking enabled by default
        CheckSyntax: true,

        // Semantic checking enabled by default
        CheckTableReferences:  true,
        CheckColumnReferences: true,
        CheckSchemaReferences: true,

        // Warnings enabled by default
        WarnOnSelectStar:      true,
        WarnOnNullComparison:  true,
        WarnOnUnusedAlias:     false, // Can be noisy
        WarnOnImplicitJoin:    true,
        WarnOnAmbiguousColumn: true,

        // Style rules
        CheckReservedWordCase: RuleSeverityOff, // Off by default
        PreferredKeywordCase:  "upper",
        CheckMissingSemicolon: RuleSeverityOff, // Off by default

        // Defaults
        MaxDiagnostics: 100,
        LintOnChange:   true,
        LintOnSave:     true,
        DebugMode:      false,
    }
}

// IsEnabled checks if a rule with given severity is enabled
func (c *Config) IsRuleEnabled(severity RuleSeverity) bool {
    return severity != RuleSeverityOff
}

// GetDiagnosticSeverity converts rule severity to diagnostic severity
func GetDiagnosticSeverity(ruleSeverity RuleSeverity) diagnostic.DiagnosticSeverity {
    switch ruleSeverity {
    case RuleSeverityError:
        return diagnostic.SeverityError
    case RuleSeverityWarning:
        return diagnostic.SeverityWarning
    case RuleSeverityInfo:
        return diagnostic.SeverityInfo
    case RuleSeverityHint:
        return diagnostic.SeverityHint
    default:
        return diagnostic.SeverityWarning
    }
}

