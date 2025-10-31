# SQL Linter Documentation

The sqls language server includes a comprehensive linting system that validates SQL code for syntax errors, semantic issues, and style violations.

## Features

### Error Detection

1. **Syntax Errors**
   - Unclosed strings and parentheses
   - Invalid token sequences
   - Malformed SQL statements

2. **Semantic Errors**
   - Invalid table references
   - Invalid column references
   - Ambiguous column references (when multiple tables have the same column)
   - Invalid schema/database names

3. **Warnings**
   - Use of `SELECT *`
   - Incorrect NULL comparisons (`= NULL` instead of `IS NULL`)
   - Implicit joins (comma-separated tables in FROM clause)
   - Unused aliases
   - Ambiguous column references

4. **Style Hints**
   - Reserved word case conventions
   - Missing semicolons

## Configuration

The linter can be configured in your `~/.config/sqls/config.yml` file under the `linter` section:

```yaml
linter:
  # Enable/disable the linter entirely
  enabled: true

  # Syntax validation
  checkSyntax: true

  # Semantic validation
  checkTableReferences: true
  checkColumnReferences: true
  checkSchemaReferences: true

  # Semantic warnings
  warnOnSelectStar: true
  warnOnNullComparison: true
  warnOnUnusedAlias: false
  warnOnImplicitJoin: true
  warnOnAmbiguousColumn: true

  # Style rules
  checkReservedWordCase: "off"  # Options: "error", "warning", "info", "hint", "off"
  preferredKeywordCase: "upper"  # Options: "upper" or "lower"
  checkMissingSemicolon: "off"   # Options: "error", "warning", "info", "hint", "off"

  # Advanced options
  maxDiagnostics: 100
  lintOnChange: true
  lintOnSave: true
```

## Configuration Options

### Core Settings

- **`enabled`** (boolean, default: `true`)
  - Master switch to enable/disable all linting

- **`lintOnChange`** (boolean, default: `true`)
  - Run linter as you type

- **`lintOnSave`** (boolean, default: `true`)
  - Run linter when you save the file

- **`maxDiagnostics`** (integer, default: `100`)
  - Maximum number of diagnostics to report per file

### Syntax Checking

- **`checkSyntax`** (boolean, default: `true`)
  - Validate SQL syntax for basic errors

### Semantic Checking

- **`checkTableReferences`** (boolean, default: `true`)
  - Validate that referenced tables exist in the database

- **`checkColumnReferences`** (boolean, default: `true`)
  - Validate that referenced columns exist in their tables

- **`checkSchemaReferences`** (boolean, default: `true`)
  - Validate that referenced schemas exist

### Semantic Warnings

- **`warnOnSelectStar`** (boolean, default: `true`)
  - Warn when using `SELECT *` instead of explicit column names

- **`warnOnNullComparison`** (boolean, default: `true`)
  - Warn when comparing with NULL using `=` or `!=` instead of `IS NULL`/`IS NOT NULL`

- **`warnOnUnusedAlias`** (boolean, default: `false`)
  - Warn when a table/column alias is defined but never used

- **`warnOnImplicitJoin`** (boolean, default: `true`)
  - Warn when using comma-separated tables instead of explicit JOIN syntax

- **`warnOnAmbiguousColumn`** (boolean, default: `true`)
  - Warn when a column name could refer to multiple tables

### Style Rules

- **`checkReservedWordCase`** (string, default: `"off"`)
  - Check that SQL keywords follow a consistent case convention
  - Options: `"error"`, `"warning"`, `"info"`, `"hint"`, `"off"`

- **`preferredKeywordCase`** (string, default: `"upper"`)
  - Preferred case for SQL keywords
  - Options: `"upper"` or `"lower"`

- **`checkMissingSemicolon`** (string, default: `"off"`)
  - Check for missing semicolons at the end of statements
  - Options: `"error"`, `"warning"`, `"info"`, `"hint"`, `"off"`

## Diagnostic Codes

The linter reports diagnostics with specific codes for easy identification:

### Syntax Errors
- `syntax-error` - General syntax error
- `unclosed-string` - Unclosed string literal
- `unclosed-parenthesis` - Unclosed parenthesis
- `invalid-token` - Invalid token in SQL
- `missing-clause` - Missing required SQL clause

### Semantic Errors
- `table-not-found` - Referenced table does not exist
- `column-not-found` - Referenced column does not exist in table
- `ambiguous-column` - Column name exists in multiple tables
- `invalid-schema` - Referenced schema does not exist
- `invalid-database` - Referenced database does not exist

### Warnings
- `select-star` - Use of SELECT *
- `null-comparison` - Comparing with NULL using = or !=
- `implicit-join` - Implicit join detected
- `unused-alias` - Defined but unused alias
- `case-mismatch` - Case sensitivity mismatch

### Style Hints
- `reserved-word-case` - Reserved word case convention
- `missing-semicolon` - Missing semicolon at end of statement

## Examples

### Example 1: Invalid Table Reference

**SQL:**
```sql
SELECT * FROM non_existent_table;
```

**Diagnostic:**
- **Error**: Table 'non_existent_table' not found
- **Code**: `table-not-found`
- **Location**: `non_existent_table`

### Example 2: Invalid Column Reference

**SQL:**
```sql
SELECT invalid_column FROM users;
```

**Diagnostic:**
- **Error**: Column 'invalid_column' not found in table 'users'
- **Code**: `column-not-found`
- **Location**: `invalid_column`

### Example 3: NULL Comparison Warning

**SQL:**
```sql
SELECT * FROM users WHERE email = NULL;
```

**Diagnostic:**
- **Warning**: Comparing with NULL using '=' or '!=', use 'IS NULL' or 'IS NOT NULL'
- **Code**: `null-comparison`
- **Location**: `=`

### Example 4: SELECT * Warning

**SQL:**
```sql
SELECT * FROM users;
```

**Diagnostic:**
- **Warning**: Use of SELECT * is discouraged, specify columns explicitly
- **Code**: `select-star`
- **Location**: `*`

### Example 5: Ambiguous Column

**SQL:**
```sql
SELECT id FROM users, orders;
```

**Diagnostic:**
- **Warning**: Column 'id' is ambiguous, found in tables: users, orders
- **Code**: `ambiguous-column`
- **Location**: `id`

### Example 6: Implicit Join Warning

**SQL:**
```sql
SELECT * FROM users, orders WHERE users.id = orders.user_id;
```

**Diagnostic:**
- **Warning**: Implicit join detected, consider using explicit JOIN syntax
- **Code**: `implicit-join`
- **Location**: `,` (comma between tables)

## Integration with Monaco Editor

The linter is fully compatible with Monaco Editor and will automatically publish diagnostics through the LSP protocol. Diagnostics will appear as:

- **Red squiggly lines** for errors
- **Yellow squiggly lines** for warnings
- **Blue squiggly lines** for information
- **Dotted lines** for hints

You can hover over the marked code to see the full diagnostic message and code.

## Performance Considerations

- The linter is designed to be fast and non-blocking
- Linting runs asynchronously and won't block editor operations
- Database cache is used for table/column validation to avoid repeated queries
- Set `lintOnChange: false` if you experience performance issues with large files

## Troubleshooting

### Linter not working

1. Ensure linter is enabled: `linter.enabled: true`
2. Check that you have a valid database connection
3. Verify the database cache has been populated (may take a moment on first connection)

### Too many false positives

1. Adjust individual rule settings
2. Disable specific warnings: `warnOnSelectStar: false`
3. Set style rules to "off" if not needed

### Column/table validation not working

1. Ensure you have a valid database connection
2. Wait for the database cache to populate
3. Check `checkTableReferences` and `checkColumnReferences` are enabled

---

# Implementation Details

## Architecture

The linter follows a modular, layered architecture:

```
internal/linter/
├── linter.go              # Main linter coordinator
├── config.go              # Configuration types and defaults
├── diagnostic.go          # Diagnostic types and builders
└── validator/
    ├── syntax_validator.go    # Syntax validation
    ├── table_validator.go     # Table reference validation
    ├── column_validator.go    # Column reference validation
    └── style_validator.go     # Style checking
```

## Components

### 1. Diagnostic System (`diagnostic.go`)

**Purpose:** Defines the diagnostic types, severity levels, and builders for creating diagnostics.

**Key Types:**
- `Diagnostic` - Represents a single linting issue
- `DiagnosticSeverity` - Error, Warning, Info, Hint levels
- `DiagnosticCode` - Unique codes for each diagnostic type
- `DiagnosticBuilder` - Helper for constructing diagnostics

**Features:**
- LSP-compatible diagnostic format
- Position tracking using token positions
- Automatic conversion to LSP protocol format
- Helper functions for formatted error messages

### 2. Configuration System (`config.go`)

**Purpose:** Manages linter configuration with sensible defaults.

**Key Features:**
- Toggle individual validation rules
- Configurable severity levels for style rules
- Performance tuning options (max diagnostics, lint triggers)
- Rule categories: syntax, semantic, style

**Integration:**
- Added to `internal/config/config.go` as `Linter` field
- Loaded from YAML configuration files
- Default configuration provided

### 3. Syntax Validator (`validator/syntax_validator.go`)

**Purpose:** Validates SQL syntax and detects basic structural errors.

**Checks:**
- Unclosed strings and parentheses
- Invalid token sequences (e.g., consecutive commas)
- NULL comparison issues (`= NULL` vs `IS NULL`)
- SELECT * usage warnings

**Implementation:**
- Uses AST walking for efficient validation
- Token-level analysis for structural issues
- Configurable warning levels

### 4. Table Validator (`validator/table_validator.go`)

**Purpose:** Validates table references against the database schema.

**Checks:**
- Table existence in database
- Schema existence
- Implicit join detection (comma-separated tables)
- Partial keyword filtering to prevent false positives

**Features:**
- Uses database cache for fast lookups
- Schema-qualified name support
- Case-insensitive matching
- Extract table information from queries

**Integration:**
- Leverages existing `database.DBCache`
- Uses `parseutil.ExtractTable()` for query analysis

### 5. Column Validator (`validator/column_validator.go`)

**Purpose:** Validates column references within queries.

**Checks:**
- Column existence in referenced tables
- Ambiguous column references (multiple tables with same column)
- Qualified vs unqualified column references
- Wildcard validation (`table.*`)

**Features:**
- Builds column context from query tables
- Handles table aliases
- Tracks columns available from each table
- Smart filtering to avoid false positives
- Position-based identifier tracking for reliable validation
- String literal detection (skips `'text'` and `"text"`)

**Implementation:**
- AST-based column extraction
- Context-aware validation
- Member identifier support (table.column)
- IdentifierList support for JOIN chains
- Skip table references in FROM/JOIN clauses

**Recent Fixes:**
- Position-based tracking instead of pointer comparison
- Table reference collection from FROM/JOIN clauses
- IdentifierList handling for multiple JOINs
- String literal detection for both single and double quotes

### 6. Style Validator (`validator/style_validator.go`)

**Purpose:** Enforces SQL style conventions.

**Checks:**
- Reserved word case consistency
- Missing semicolons
- Unused aliases

**Features:**
- Configurable severity levels
- Dialect-aware keyword detection
- Multiple style conventions supported

### 7. Main Linter (`linter.go`)

**Purpose:** Coordinates all validators and manages the linting process.

**Features:**
- Orchestrates all validators
- Manages validator lifecycle
- Limits diagnostic output
- Supports dynamic configuration updates

**API:**
```go
func New(config *Config, dbCache *database.DBCache, dialect dialect.Dialect) *Linter
func (l *Linter) Lint(text string) ([]lsp.Diagnostic, error)
func (l *Linter) UpdateConfig(config *Config)
func (l *Linter) UpdateDBCache(dbCache *database.DBCache)
```

## LSP Integration

### Modified Files

1. **`internal/handler/handler.go`**
   - Added `linter *linter.Linter` field to Server
   - Modified `handleTextDocumentDidOpen` to lint on open
   - Modified `handleTextDocumentDidChange` to lint on change
   - Modified `handleTextDocumentDidSave` to lint on save
   - Modified `handleTextDocumentDidClose` to clear diagnostics

2. **`internal/handler/diagnostics.go`** (NEW)
   - `publishDiagnostics()` - Sends diagnostics to LSP client
   - `lintDocument()` - Performs linting and publishes results
   - `initializeLinter()` - Initializes linter with config and cache
   - `clearDiagnostics()` - Clears diagnostics for a file
   - `updateLinterConfig()` - Updates linter configuration

3. **`internal/lsp/lsp.go`**
   - Added `DiagnosticSeverity` type and constants
   - Modified `Diagnostic` type (changed Code and Source to strings)
   - Added `PublishDiagnosticsParams` type

4. **`internal/config/config.go`**
   - Added `Linter *linter.Config` field to Config
   - Updated `NewConfig()` to initialize default linter config

## Diagnostic Flow

```
1. User types in editor (Monaco)
   ↓
2. LSP client sends textDocument/didChange
   ↓
3. Handler updates file content
   ↓
4. If lintOnChange enabled:
   - Handler calls lintDocument()
   - Linter.Lint() is called
   - Validators run in sequence
   - Diagnostics are collected
   ↓
5. Handler publishes diagnostics via textDocument/publishDiagnostics
   ↓
6. LSP client receives diagnostics
   ↓
7. Monaco editor displays squiggly lines and messages
```

## Database Integration

The linter integrates with the existing database infrastructure:

- **DBCache:** Used for table/column validation
- **DBConnection:** Provides access to schema information
- **Dialect:** Determines SQL dialect for keyword validation
- **Worker:** Background cache updates don't affect linting

## Performance

1. **Caching:** Uses existing database cache, no additional queries
2. **Async:** Linting runs asynchronously, doesn't block editor
3. **Limits:** `maxDiagnostics` prevents overwhelming output
4. **Toggles:** `lintOnChange` can be disabled for large files
5. **Incremental:** Only lints changed documents

## Error Handling

- Parse errors are caught and reported as diagnostics
- Database connection failures don't crash the linter
- Missing cache gracefully degrades to syntax-only validation
- Invalid configuration falls back to defaults

## Supported Databases

The linter works with all databases supported by sqls:
- MySQL
- PostgreSQL
- SQLite3
- MSSQL
- H2
- Vertica
- Oracle (partial)
- ClickHouse (partial)

Table/column validation requires an active database connection.

## Code Statistics

- **New Files:** 7 (including documentation)
- **Modified Files:** 4
- **Lines of Code:** ~1,500+ (excluding comments and tests)
- **Diagnostic Codes:** 20+
- **Configuration Options:** 15+

## Dependencies

No new external dependencies added. Uses existing:
- `github.com/sqls-server/sqls/ast`
- `github.com/sqls-server/sqls/parser`
- `github.com/sqls-server/sqls/token`
- `github.com/sqls-server/sqls/dialect`
- `github.com/sqls-server/sqls/internal/database`
