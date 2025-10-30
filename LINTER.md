# SQL Linter Documentation

The sqls language server now includes a comprehensive linting system that validates SQL code for syntax errors, semantic issues, and style violations.

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

## Future Enhancements

Potential future additions to the linter:

- Type checking for expressions and comparisons
- SQL injection vulnerability detection
- Performance anti-pattern detection
- Custom rule support
- Rule severity customization per rule
- Quick fixes and code actions for common issues
