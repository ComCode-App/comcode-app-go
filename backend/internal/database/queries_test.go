package database

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Account isolation lint (TEC-1107).
//
// Every sqlc query that reads or writes an account-owned table (listed in
// account_tables.txt) must scope that table by account:
//   - an account_id predicate on that table (or its alias) compared with a
//     parameter, literal, IN list, ANY(...) or subquery, or "IS NULL"
//     (unowned rows only);
//   - or an "x.account_id = y.account_id" chain ending in such a predicate
//     (accounts.id counts as an account id);
//   - or a primary-key join from a scoped row ("p.id = c.profile_id", c scoped);
//   - or, for an INSERT into the table, account_id in the column list.
//
// A join such as "a.id = p.account_id" or "p.account_id = r.account_id" with
// no filter at the end of the chain does not count: it links two rows but
// scopes neither. Neither does account_id in a SELECT list or UPDATE SET.
// The check is lexical (regexps over the query text, not a SQL parser), so it
// errs towards flagging; restructure the query or annotate it.
//
// Queries that are cross-account on purpose (token lookups, uuid lookups whose
// ownership the use case checks, workers, public reads, platform admin views)
// opt out with a comment directly above the "-- name:" line:
//
//	-- account-lint: cross-account <reason>

const accountLintMarker = "-- account-lint: cross-account"

func TestAccountQueriesReferenceAccount(t *testing.T) {
	tables := readAccountTables(t)
	files, err := filepath.Glob(filepath.Join("queries", "*.sql"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no query files: %v", err)
	}
	var checked, exempt int
	for _, file := range files {
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		queries := splitQueries(string(raw))
		for _, problem := range lintAccountQueries(file, queries, tables) {
			t.Error(problem)
		}
		for _, q := range queries {
			if len(touchedAccountTables(q.body, tables)) == 0 {
				continue
			}
			checked++
			if q.exempt {
				exempt++
			}
		}
	}
	t.Logf("account lint: %d tables, %d queries touch them, %d exempt", len(tables), checked, exempt)
}

func TestAccountLintSelfTest(t *testing.T) {
	tables := []string{"profiles", "nfc_cards"}
	cases := []struct {
		name string
		src  string
		ok   bool
	}{
		{"filtered by param", `-- name: Q :one
SELECT * FROM profiles WHERE uuid = $1 AND account_id = $2;`, true},
		{"filtered by alias and named arg", `-- name: Q :many
SELECT p.* FROM profiles p JOIN nfc_cards c ON c.profile_id = p.id
WHERE p.account_id = sqlc.arg(account_id) AND c.account_id = @account_id;`, true},
		{"param on the left", `-- name: Q :exec
DELETE FROM profiles WHERE $1 = account_id AND id = $2;`, true},
		{"insert with account_id column", `-- name: Q :one
INSERT INTO profiles (account_id, user_id, name) VALUES ($1, $2, $3) RETURNING *;`, true},
		{"unowned rows only", `-- name: Q :many
SELECT * FROM profiles WHERE account_id IS NULL;`, true},
		{"unfiltered", `-- name: Q :one
SELECT * FROM profiles WHERE uuid = $1;`, false},
		{"account_id only in SELECT list", `-- name: Q :one
SELECT id, account_id FROM profiles WHERE uuid = $1;`, false},
		{"account_id only in UPDATE SET", `-- name: Q :exec
UPDATE profiles SET account_id = $2 WHERE id = $1;`, false},
		{"join only", `-- name: Q :one
SELECT p.* FROM profiles p JOIN accounts a ON a.id = p.account_id WHERE a.uuid = $1;`, false},
		{"scope carried through account_id equality", `-- name: Q :many
SELECT p.* FROM profiles p JOIN nfc_cards c ON c.account_id = p.account_id WHERE c.account_id = $1;`, true},
		{"column-to-column only", `-- name: Q :many
SELECT p.* FROM profiles p JOIN nfc_cards c ON c.account_id = p.account_id WHERE c.uuid = $1;`, false},
		{"scope through accounts.id", `-- name: Q :one
SELECT count(*) FROM accounts a JOIN profiles p ON p.account_id = a.id WHERE a.id = $1;`, true},
		{"accounts joined by uuid only", `-- name: Q :one
SELECT count(*) FROM accounts a JOIN profiles p ON p.account_id = a.id WHERE a.uuid = $1;`, false},
		{"reached by primary key from a scoped row", `-- name: Q :many
SELECT c.*, p.uuid FROM nfc_cards c LEFT JOIN profiles p ON p.id = c.profile_id WHERE c.account_id = $1;`, true},
		{"children of a scoped row are not reached", `-- name: Q :many
SELECT c.* FROM profiles p JOIN nfc_cards c ON c.profile_id = p.id WHERE p.account_id = $1;`, false},
		{"filter on the wrong table", `-- name: Q :many
SELECT c.* FROM nfc_cards c JOIN profiles p ON p.id = c.profile_id WHERE p.account_id = $1;`, false},
		{"annotated", `-- account-lint: cross-account token lookup, the token is the secret
-- name: Q :one
SELECT * FROM profiles WHERE uuid = $1;`, true},
		{"annotation without reason", `-- account-lint: cross-account
-- name: Q :one
SELECT * FROM profiles WHERE account_id = $1;`, false},
		{"annotation not directly above", `-- account-lint: cross-account reason

-- name: Q :one
SELECT * FROM profiles WHERE uuid = $1;`, false},
		{"untracked table", `-- name: Q :one
SELECT * FROM users WHERE id = $1;`, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			problems := lintAccountQueries("self.sql", splitQueries(tc.src), tables)
			if tc.ok && len(problems) > 0 {
				t.Fatalf("expected pass, got %v", problems)
			}
			if !tc.ok && len(problems) == 0 {
				t.Fatal("expected a lint failure, got none")
			}
		})
	}
}

type namedQuery struct {
	name   string
	body   string
	exempt bool
	reason string
	// misplacedMarker is set when an account-lint comment is not directly
	// above this query's "-- name:" line.
	misplacedMarker bool
}

var nameRe = regexp.MustCompile(`^--\s*name:\s*(\w+)`)

// splitQueries cuts a sqlc query file into named queries. An account-lint
// marker applies only when the very next line is the "-- name:" line.
func splitQueries(src string) []namedQuery {
	var out []namedQuery
	var cur *namedQuery
	var pending *string
	stray := false
	for _, line := range strings.Split(src, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, accountLintMarker) {
			r := strings.TrimSpace(strings.TrimPrefix(trimmed, accountLintMarker))
			pending = &r
			continue
		}
		if m := nameRe.FindStringSubmatch(trimmed); m != nil {
			if cur != nil {
				out = append(out, *cur)
			}
			cur = &namedQuery{name: m[1], misplacedMarker: stray}
			stray = false
			if pending != nil {
				cur.exempt, cur.reason = true, *pending
				pending = nil
			}
			continue
		}
		if pending != nil {
			// The marker was not directly above a "-- name:" line.
			stray = true
			pending = nil
		}
		if cur != nil {
			cur.body += line + "\n"
		}
	}
	if cur != nil {
		out = append(out, *cur)
	}
	return out
}

func lintAccountQueries(file string, queries []namedQuery, tables []string) []string {
	var problems []string
	fix := fmt.Sprintf("add an account_id predicate on that table (e.g. \"AND <alias>.account_id = sqlc.arg(account_id)\") or, if the query is cross-account on purpose, put %q directly above its \"-- name:\" line", accountLintMarker+" <reason>")
	for _, q := range queries {
		if q.misplacedMarker {
			problems = append(problems, fmt.Sprintf("%s: query %s: an %q comment must be directly above the \"-- name:\" line", file, q.name, accountLintMarker))
		}
		if q.exempt {
			if q.reason == "" {
				problems = append(problems, fmt.Sprintf("%s: query %s: account-lint exemption needs a reason", file, q.name))
			}
			continue
		}
		for _, table := range unscopedAccountTables(q.body, tables) {
			problems = append(problems, fmt.Sprintf("%s: query %s touches account-owned table %s without an account_id filter on it; %s", file, q.name, table, fix))
		}
	}
	return problems
}

// tableUse is one appearance of an account-owned table in a query.
type tableUse struct {
	table  string
	alias  string // empty when the table is not aliased
	insert bool   // INSERT INTO target
	cols   string // INSERT column list
}

var sqlKeywords = map[string]bool{
	"where": true, "on": true, "set": true, "join": true, "left": true, "right": true,
	"inner": true, "outer": true, "full": true, "cross": true, "using": true, "group": true,
	"order": true, "limit": true, "offset": true, "returning": true, "values": true,
	"select": true, "for": true, "having": true, "union": true, "natural": true,
	"lateral": true, "default": true, "on_conflict": true, "and": true, "or": true,
	"window": true, "except": true, "intersect": true, "fetch": true,
}

func normalizeSQL(sql string) string {
	var b strings.Builder
	for _, line := range strings.Split(sql, "\n") {
		if i := strings.Index(line, "--"); i >= 0 {
			line = line[:i]
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	s := strings.ToLower(b.String())
	// Drop string literals so 'account_id' text cannot satisfy the lint.
	s = regexp.MustCompile(`'(?:[^']|'')*'`).ReplaceAllString(s, "''")
	return s
}

func findTableUses(sql string, table string) []tableUse {
	re := regexp.MustCompile(`\b(from|join|update|into|,)\s+(?:only\s+)?(?:public\.)?` + table + `\b(?:\s+(?:as\s+)?([a-z_][a-z0-9_]*))?(?:\s*\(([^)]*)\))?`)
	var uses []tableUse
	for _, m := range re.FindAllStringSubmatch(sql, -1) {
		u := tableUse{table: table}
		if m[2] != "" && !sqlKeywords[m[2]] {
			u.alias = m[2]
		}
		if m[1] == "into" {
			u.insert = true
			u.cols = m[3]
		}
		uses = append(uses, u)
	}
	return uses
}

func touchedAccountTables(body string, tables []string) []string {
	sql := normalizeSQL(body)
	var out []string
	for _, table := range tables {
		if len(findTableUses(sql, table)) > 0 {
			out = append(out, table)
		}
	}
	return out
}

// stripSetClauses removes the assignment list of UPDATE ... SET and
// ON CONFLICT DO UPDATE SET, so "SET account_id = $2" is not read as a filter.
var setClauseRe = regexp.MustCompile(`(?s)\bset\b.*?(\bwhere\b|\bfrom\b|\breturning\b|;|$)`)

const (
	rhsValue = `(?:\$\d+|@[a-z_]\w*|sqlc\.n?arg\s*\(|sqlc\.slice\s*\(|any\s*\(|\(\s*select\b|\d+\b|coalesce\s*\(\s*(?:\$\d+|@|sqlc\.))`
	opFwd    = `\s*(?:=\s*` + rhsValue + `|in\s*\(|is\s+null\b|is\s+not\s+distinct\s+from\s*` + rhsValue + `)`
	lhsValue = `(?:\$\d+|@[a-z_]\w*|sqlc\.n?arg\s*\([^)]*\))\s*=\s*`
)

var (
	qualifiedRefRe = regexp.MustCompile(`([a-z_][a-z0-9_]*)\.account_id\b`)
	refPattern     = `(?:[a-z_][a-z0-9_]*\.)?account_id\b`
	refEqRefRe     = regexp.MustCompile(`(?:^|[^\w.])(` + refPattern + `)\s*=\s*(` + refPattern + `)`)
	anchorFwdRe    = regexp.MustCompile(`(?:^|[^\w.])(` + refPattern + `)` + opFwd)
	anchorBackRe   = regexp.MustCompile(lhsValue + `(` + refPattern + `)`)
	insertColsRe   = regexp.MustCompile(`\baccount_id\b`)
	// Primary-key join: "x.id = y.col" (x=1, y=2) or "y.col = x.id" (y=3, x=4).
	pkJoinRe = regexp.MustCompile(`(?:([a-z_][a-z0-9_]*)\.id\s*=\s*([a-z_][a-z0-9_]*)\.[a-z_][a-z0-9_]*|([a-z_][a-z0-9_]*)\.[a-z_][a-z0-9_]*\s*=\s*([a-z_][a-z0-9_]*)\.id)\b`)
)

// qualifier returns the part before ".account_id", or "" for a bare column.
func qualifier(ref string) string {
	if m := qualifiedRefRe.FindStringSubmatch(ref); m != nil {
		return m[1]
	}
	return ""
}

// unscopedAccountTables returns the account-owned tables in body that are not
// scoped by account. A use is scoped when its account_id is anchored to a
// value (parameter, literal, IN list, subquery, IS NULL), directly or through
// a chain of account_id = account_id equalities that ends in such an anchor
// (p.account_id = c.account_id AND c.account_id = $1). Equalities alone, with
// no anchor, scope nothing.
func unscopedAccountTables(body string, tables []string) []string {
	sql := normalizeSQL(body)
	filterSQL := setClauseRe.ReplaceAllStringFunc(sql, func(m string) string {
		return " " + setClauseRe.FindStringSubmatch(m)[1]
	})
	var all []tableUse
	for _, table := range tables {
		all = append(all, findTableUses(sql, table)...)
	}
	filterSQL = accountsIDAsAccountID(filterSQL, all)

	// Union-find over qualifiers ("" is the bare column).
	parent := map[string]string{}
	var find func(string) string
	find = func(x string) string {
		if _, ok := parent[x]; !ok {
			parent[x] = x
		}
		if parent[x] != x {
			parent[x] = find(parent[x])
		}
		return parent[x]
	}
	for _, m := range refEqRefRe.FindAllStringSubmatch(filterSQL, -1) {
		parent[find(qualifier(m[1]))] = find(qualifier(m[2]))
	}
	anchored := map[string]bool{}
	for _, m := range anchorFwdRe.FindAllStringSubmatch(filterSQL, -1) {
		anchored[qualifier(m[1])] = true
	}
	for _, m := range anchorBackRe.FindAllStringSubmatch(filterSQL, -1) {
		anchored[qualifier(m[1])] = true
	}
	roots := map[string]bool{}
	for q := range anchored {
		roots[find(q)] = true
	}
	isAnchored := func(q string) bool { return roots[find(q)] }

	single := len(all) == 1
	scoped := map[string]bool{} // qualifier (alias or table name) -> scoped
	for _, u := range all {
		if isAnchored(u.table) || (u.alias != "" && isAnchored(u.alias)) || (single && isAnchored("")) {
			scoped[u.table], scoped[u.alias] = true, true
		}
	}
	// A row joined by its primary key from a scoped row ("p.id = c.profile_id"
	// with c scoped) is reached through that row: at most one row per scoped
	// row, chosen by the scoped row. The reverse direction (many rows pointing
	// at a scoped row, "r.card_id = c.id") is not, and needs its own filter.
	for changed := true; changed; {
		changed = false
		for _, m := range pkJoinRe.FindAllStringSubmatch(filterSQL, -1) {
			for _, pair := range [][2]string{{m[1], m[2]}, {m[4], m[3]}} {
				target, from := pair[0], pair[1]
				if target == "" || from == "" || !scoped[from] || scoped[target] {
					continue
				}
				for _, u := range all {
					if u.alias == target || (u.alias == "" && u.table == target) {
						scoped[target] = true
						changed = true
					}
				}
			}
		}
	}
	delete(scoped, "")
	var bad []string
	seen := map[string]bool{}
	for _, u := range all {
		if u.insert && insertColsRe.MatchString(u.cols) {
			continue
		}
		name := u.alias
		if name == "" {
			name = u.table
		}
		if scoped[name] {
			continue
		}
		if !seen[u.table] {
			seen[u.table] = true
			bad = append(bad, u.table)
		}
	}
	return bad
}

// accountsIDAsAccountID rewrites "<accounts alias>.id" to
// "<accounts alias>.account_id", so accounts.id takes part in the same
// anchor/equality reasoning: "m.account_id = a.id AND a.id = $1" scopes m,
// while "m.account_id = a.id" with a only matched by uuid or slug does not.
// Aliases that also name an account-owned table are left alone.
func accountsIDAsAccountID(sql string, owned []tableUse) string {
	taken := map[string]bool{}
	for _, u := range owned {
		taken[u.alias] = true
		taken[u.table] = true
	}
	names := []string{"accounts"}
	for _, u := range findTableUses(sql, "accounts") {
		if u.alias != "" && !taken[u.alias] {
			names = append(names, u.alias)
		}
	}
	for _, n := range names {
		sql = regexp.MustCompile(`(^|[^\w.])`+regexp.QuoteMeta(n)+`\.id\b`).ReplaceAllString(sql, "${1}"+n+".account_id")
	}
	return sql
}

func readAccountTables(t *testing.T) []string {
	t.Helper()
	raw, err := os.ReadFile("account_tables.txt")
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out = append(out, line)
	}
	if len(out) == 0 {
		t.Fatal("account_tables.txt is empty")
	}
	return out
}
