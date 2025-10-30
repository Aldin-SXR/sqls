-- SQL Linter Test File
-- This file demonstrates various linting features

-- === VALID QUERIES ===
-- These should not produce any errors

-- Simple SELECT with explicit columns
SELECT id, name, email FROM users;

-- JOIN with proper syntax
SELECT u.id, u.name, o.order_id
FROM users u
INNER JOIN orders o ON u.id = o.user_id;

-- Proper NULL check
SELECT * FROM users WHERE deleted_at IS NULL;

-- === SYNTAX ERRORS ===

-- Unclosed string (will produce syntax error)
-- SELECT * FROM users WHERE name = 'John

-- Invalid token sequence (consecutive commas)
-- SELECT id,, name FROM users;

-- === SEMANTIC ERRORS ===

-- Non-existent table (will produce error if linter is connected to DB)
SELECT * FROM non_existent_table;

-- Non-existent column (will produce error if linter is connected to DB)
SELECT invalid_column FROM users;

-- Invalid schema reference
SELECT * FROM invalid_schema.users;

-- === WARNINGS ===

-- SELECT * usage (warning: select-star)
SELECT * FROM users;

-- Incorrect NULL comparison (warning: null-comparison)
SELECT * FROM users WHERE email = NULL;

-- Implicit join (warning: implicit-join)
SELECT * FROM users, orders WHERE users.id = orders.user_id;

-- Ambiguous column reference (warning if both tables have 'id')
SELECT id FROM users, orders WHERE users.id = orders.user_id;

-- === STYLE ISSUES ===

-- Mixed case keywords (if checkReservedWordCase is enabled)
select id, name from users where id = 1;
SELECT id, name FROM users WHERE id = 1;

-- Missing semicolon (if checkMissingSemicolon is enabled)
SELECT * FROM users

-- === COMPLEX EXAMPLES ===

-- Multiple issues in one query
select * from non_existent_table, users where email = NULL

-- Correct version of above query
SELECT u.email
FROM users u
WHERE u.email IS NOT NULL;

-- Subquery with proper aliasing
SELECT u.id, u.name,
       (SELECT COUNT(*) FROM orders o WHERE o.user_id = u.id) as order_count
FROM users u;

-- Common Table Expression (CTE)
WITH active_users AS (
    SELECT id, name, email
    FROM users
    WHERE status = 'active'
)
SELECT * FROM active_users;

-- === BEST PRACTICES EXAMPLES ===

-- Good: Explicit columns, proper JOIN, qualified columns
SELECT
    u.id,
    u.name,
    u.email,
    o.order_id,
    o.total_amount
FROM users u
INNER JOIN orders o ON u.id = o.user_id
WHERE u.deleted_at IS NULL
  AND o.status = 'completed';

-- Good: Using aliases to avoid ambiguity
SELECT
    u.id AS user_id,
    u.name AS user_name,
    o.id AS order_id
FROM users u
LEFT JOIN orders o ON u.id = o.user_id;

-- Good: Proper NULL handling
SELECT name, email
FROM users
WHERE email IS NOT NULL
  AND deleted_at IS NULL;

-- === LINTER CONFIGURATION EXAMPLES ===

-- To disable specific warnings, update your config.yml:
-- linter:
--   warnOnSelectStar: false      # Allow SELECT *
--   warnOnImplicitJoin: false    # Allow comma joins
--   checkReservedWordCase: "off" # Don't check keyword case

-- To make the linter more strict:
-- linter:
--   checkReservedWordCase: "error"
--   checkMissingSemicolon: "warning"
--   warnOnUnusedAlias: true
