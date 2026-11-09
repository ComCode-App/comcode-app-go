package legacymigration

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/storage"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/testutil/pgtest"
)

type ownersFixture struct {
	db       *pgtest.DB
	repo     *Postgres
	sync     SyncOptions
	opts     OwnersOptions
	prefix   string
	existing pgtest.User
}

func newOwnersFixture(t *testing.T) ownersFixture {
	t.Helper()
	d := pgtest.Open(t)
	dir := t.TempDir()
	prefix := "owners" + pgtest.Unique()
	existing := d.CreateUser(t)
	var existingEmail string
	if err := d.Pool.QueryRow(context.Background(), `SELECT email FROM users WHERE id=$1`, existing.ID).Scan(&existingEmail); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "users.csv")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	w := csv.NewWriter(f)
	header := []string{
		"user_id",
		"uniqid_id",
		"username",
		"email",
		"name",
		"companies",
		"vcard",
		"active",
		"deactivated",
		"bank_accounts",
		"background_type",
		"background_value",
	}
	if err = w.Write(header); err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= 10; i++ {
		org := ""
		workEmail := ""
		title := "Sales"
		active := "1"
		if i <= 3 || i == 7 {
			org = "Coldwell Banker Kobi"
		}
		if i >= 4 && i <= 6 {
			org = "Coldwell Banker Life"
		}
		if i <= 6 {
			workEmail = "agent@cb.com.tr"
		}
		if i == 2 {
			title = "Broker/Owner"
		}
		if i == 10 {
			active = "0"
		}
		email := fmt.Sprintf("%s-%d@gmail.com", prefix, i)
		if i == 8 {
			email = strings.ToUpper(existingEmail)
		}
		v, _ := json.Marshal(map[string]string{
			"organization": org,
			"workemail":    workEmail,
			"title":        title,
			"firstname":    "Name",
			"lastname":     "Surname",
			"workstreet":   "Street",
			"cellphone":    "123",
		})
		id := fmt.Sprintf("%s-%02d", prefix, i)
		row := []string{
			id,
			id,
			id,
			email,
			"Legacy Name",
			org,
			string(v),
			active,
			"0",
			`[{"iban":"TR123","account_name":"Name"}]`,
			"color",
			"#123abc",
		}
		if err = w.Write(row); err != nil {
			t.Fatal(err)
		}
	}
	w.Flush()
	if err = w.Error(); err != nil {
		t.Fatal(err)
	}
	if err = f.Close(); err != nil {
		t.Fatal(err)
	}
	fixture := ownersFixture{db: d, repo: NewPostgres(d.Pool), prefix: prefix, existing: existing,
		sync: SyncOptions{
			Options:  options(),
			UsersCSV: path,
			StateDir: filepath.Join(dir, "state"),
			Media:    &MediaSync{Driver: storage.NewMemory()},
		},
		opts: OwnersOptions{UsersCSV: path}}
	if _, err = Sync(
		context.Background(),
		fixture.repo,
		fixture.sync,
		&bytes.Buffer{},
	); err != nil {
		t.Fatal(err)
	}
	return fixture
}

func ownerSnapshot(t *testing.T, d *pgtest.DB) string {
	t.Helper()
	// Compare every account/actor value in the affected tables, plus all users/accounts/memberships.
	var snapshot string
	err := d.Pool.QueryRow(context.Background(), `SELECT jsonb_build_object(
  'profiles',(SELECT jsonb_agg(jsonb_build_array(id,account_id,user_id) ORDER BY id) FROM profiles),
  'routes',(SELECT jsonb_agg(jsonb_build_array(id,account_id,owner_user_id) ORDER BY id) FROM routes),
  'banks',(SELECT jsonb_agg(jsonb_build_array(id,account_id,user_id) ORDER BY id) FROM user_bank_accounts),
  'backgrounds',(SELECT jsonb_agg(jsonb_build_array(id,account_id,created_by) ORDER BY id) FROM backgrounds),
  'users',(SELECT jsonb_agg(to_jsonb(u) ORDER BY id) FROM users u),
  'accounts',(SELECT jsonb_agg(to_jsonb(a) ORDER BY id) FROM accounts a),
  'members',(SELECT jsonb_agg(to_jsonb(m) ORDER BY id) FROM account_members m))::text`).Scan(&snapshot)
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func TestOwnersFullCycleRealPostgres(t *testing.T) {
	f := newOwnersFixture(t)
	ctx := context.Background()
	before := ownerSnapshot(t, f.db)
	f.opts.DryRun = true
	var dryReport bytes.Buffer
	planned, err := Owners(
		ctx,
		f.repo,
		f.opts,
		&dryReport,
	)
	if err != nil {
		t.Fatal(err)
	}
	if planned.Individuals != 2 || planned.Companies != 2 || planned.CompanyUsers != 7 || planned.Skipped != 1 {
		t.Fatalf("plan %+v", planned)
	}
	if got := ownerSnapshot(t, f.db); got != before {
		t.Fatal("dry-run changed DB")
	}
	f.opts.DryRun = false
	var applyReport bytes.Buffer
	applied, err := Owners(
		ctx,
		f.repo,
		f.opts,
		&applyReport,
	)
	if err != nil {
		t.Fatal(err)
	}
	planned.Batch = applied.Batch
	// Dry-run cannot count roles for users it has not created yet.
	if planned.RolesAssigned != 0 || applied.RolesAssigned == 0 {
		t.Fatalf("roles planned=%d applied=%d", planned.RolesAssigned, applied.RolesAssigned)
	}
	planned.RolesAssigned = applied.RolesAssigned
	if !reflect.DeepEqual(planned, applied) || dryReport.String() != applyReport.String() {
		t.Fatalf("dry/apply mismatch: %+v %+v", planned, applied)
	}
	var unroled int
	if err = f.db.Pool.QueryRow(ctx, `SELECT count(*) FROM legacy_owner_items i
WHERE i.batch_id=$1 AND i.created_user AND NOT EXISTS (
  SELECT 1 FROM user_roles ur JOIN roles r ON r.id=ur.role_id WHERE ur.user_id=i.user_id AND r.slug='subscriber')`,
		applied.Batch).Scan(&unroled); err != nil || unroled != 0 {
		t.Fatalf("created users without subscriber role=%d err=%v", unroled, err)
	}
	// A batch applied before role assignment existed is backfilled by the next run.
	if _, err = f.db.Pool.Exec(ctx, `DELETE FROM user_roles WHERE user_id IN (
  SELECT user_id FROM legacy_owner_items WHERE batch_id=$1 AND created_user)`, applied.Batch); err != nil {
		t.Fatal(err)
	}
	backfill, err := Owners(ctx, f.repo, f.opts, &bytes.Buffer{})
	if err != nil || backfill.Assigned != 0 || backfill.RolesAssigned != applied.RolesAssigned {
		t.Fatalf("backfill %+v err=%v", backfill, err)
	}
	var companies, owners, personal, createdUsers int
	err = f.db.Pool.QueryRow(ctx, `SELECT
  (SELECT count(DISTINCT account_id) FROM legacy_owner_items WHERE batch_id=$1 AND legacy_user_id < $2),
  (SELECT count(*) FROM account_members WHERE role='owner' AND account_id IN (SELECT account_id FROM legacy_owner_items WHERE batch_id=$1)),
  (SELECT count(*) FROM accounts WHERE type='personal' AND personal_user_id IN (SELECT user_id FROM legacy_owner_items WHERE batch_id=$1 AND legacy_user_id < $2)),
  (SELECT count(*) FROM users WHERE id IN (SELECT user_id FROM legacy_owner_items WHERE batch_id=$1 AND created_user)
    AND status='pending' AND email_verified_at IS NULL AND last_login_at IS NULL AND password_hash LIKE '$argon2id$%')`, applied.Batch, f.prefix+"-08").Scan(
		&companies,
		&owners,
		&personal,
		&createdUsers,
	)
	if err != nil || companies != 2 || owners != 4 || personal != 0 || createdUsers != 8 {
		t.Fatalf(
			"accounts companies=%d owners=%d personal=%d users=%d: %v",
			companies,
			owners,
			personal,
			createdUsers,
			err,
		)
	}
	var reused bool
	err = f.db.Pool.QueryRow(
		ctx,
		`SELECT user_id=$2 AND account_id=$3 AND NOT created_user AND NOT created_account AND NOT created_membership FROM legacy_owner_items WHERE batch_id=$1 AND legacy_user_id=$4`,
		applied.Batch,
		f.existing.ID,
		f.existing.AccountID,
		f.prefix+"-08",
	).Scan(&reused)
	if err != nil || !reused {
		t.Fatal("existing user/personal account not reused", err)
	}
	var correctOwner bool
	err = f.db.Pool.QueryRow(
		ctx,
		`SELECT m.role='owner' FROM account_members m JOIN legacy_owner_items i ON i.membership_id=m.id WHERE i.batch_id=$1 AND i.legacy_user_id=$2`,
		applied.Batch,
		f.prefix+"-02",
	).Scan(&correctOwner)
	if err != nil || !correctOwner {
		t.Fatal("title owner selection", err)
	}
	assigned := ownerSnapshot(t, f.db)
	again, err := Owners(
		ctx,
		f.repo,
		f.opts,
		&bytes.Buffer{},
	)
	if err != nil || again.Assigned != 0 || again.Batch != "" || ownerSnapshot(t, f.db) != assigned {
		t.Fatalf("rerun %+v %v", again, err)
	}
	synced, err := Sync(
		ctx,
		f.repo,
		f.sync,
		&bytes.Buffer{},
	)
	if err != nil || synced.Creates != 0 || synced.Skips != 10 || ownerSnapshot(t, f.db) != assigned {
		t.Fatalf("sync after assignment %+v %v", synced, err)
	}
	// Exercise a real update, not only the content-hash fast path.
	data, err := os.ReadFile(f.sync.UsersCSV)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(f.sync.UsersCSV, bytes.ReplaceAll(data, []byte("Legacy Name"), []byte("Updated Name")), 0600); err != nil {
		t.Fatal(err)
	}
	synced, err = Sync(
		ctx,
		f.repo,
		f.sync,
		&bytes.Buffer{},
	)
	if err != nil || synced.Creates != 0 || synced.Updates != 10 || ownerSnapshot(t, f.db) != assigned {
		t.Fatalf("changed sync %+v %v", synced, err)
	}
	rollback, err := RollbackOwners(ctx, f.repo, OwnersRollbackOptions{Batch: applied.Batch})
	if err != nil || rollback.Batches != 1 || len(rollback.Conflicts) > 0 || len(rollback.RetainedAccounts) > 0 {
		t.Fatalf("rollback %+v %v", rollback, err)
	}
	if got := ownerSnapshot(t, f.db); got != before {
		t.Fatalf("restore differs\nbefore=%s\nafter=%s", before, got)
	}
	rollback, err = RollbackOwners(ctx, f.repo, OwnersRollbackOptions{Batch: applied.Batch})
	if err != nil || rollback.Batches != 0 || rollback.Restored != 0 || ownerSnapshot(t, f.db) != before {
		t.Fatalf("second rollback %+v %v", rollback, err)
	}
}

func TestOwnersOverridesAndRollbackConflicts(t *testing.T) {
	f := newOwnersFixture(t)
	ctx := context.Background()
	override := filepath.Join(t.TempDir(), "overrides.csv")
	csvText := "legacy_user_id,group\n" + f.prefix + "-01,solo\n" + f.prefix + "-02,holding\n" + f.prefix + "-08,manual\n" + f.prefix + "-09,manual\n"
	if err := os.WriteFile(override, []byte(csvText), 0600); err != nil {
		t.Fatal(err)
	}
	f.opts.OverridesCSV = override
	applied, err := Owners(
		ctx,
		f.repo,
		f.opts,
		&bytes.Buffer{},
	)
	if err != nil {
		t.Fatal(err)
	}
	if applied.Skipped != 2 || applied.Companies != 3 || applied.Individuals != 1 {
		t.Fatalf("overrides %+v", applied)
	}
	var account, user, profile int64
	err = f.db.Pool.QueryRow(
		ctx,
		`SELECT account_id,user_id,profile_id FROM legacy_owner_items WHERE batch_id=$1 AND legacy_user_id=$2`,
		applied.Batch,
		f.prefix+"-01",
	).Scan(&account, &user, &profile)
	if err != nil {
		t.Fatal(err)
	}
	// A later profile must survive rollback, and a logged-in user must survive too.
	if _, err = f.db.Pool.Exec(
		ctx,
		`INSERT INTO profiles(account_id,user_id) VALUES($1,$2)`,
		account,
		user,
	); err != nil {
		t.Fatal(err)
	}
	if _, err = f.db.Pool.Exec(ctx, `UPDATE users SET status='active',last_login_at=now() WHERE id=$1`, user); err != nil {
		t.Fatal(err)
	}
	// Creator changed after migration: do not overwrite that change.
	if _, err = f.db.Pool.Exec(
		ctx,
		`UPDATE profiles SET user_id=$2 WHERE id=$1`,
		profile,
		f.existing.ID,
	); err != nil {
		t.Fatal(err)
	}
	result, err := RollbackOwners(ctx, f.repo, OwnersRollbackOptions{Batch: applied.Batch})
	if err != nil || len(result.Conflicts) != 1 || len(result.RetainedAccounts) != 1 || len(result.RetainedUsers) != 1 {
		t.Fatalf("conflicts %+v %v", result, err)
	}
	var preserved bool
	err = f.db.Pool.QueryRow(
		ctx,
		`SELECT user_id=$2 FROM profiles WHERE id=$1`,
		profile,
		f.existing.ID,
	).Scan(&preserved)
	if err != nil || !preserved {
		t.Fatal("conflicting creator overwritten", err)
	}
}

func TestOwnersInvalidInputIsAtomic(t *testing.T) {
	f := newOwnersFixture(t)
	ctx := context.Background()
	before := ownerSnapshot(t, f.db)
	data, err := os.ReadFile(f.opts.UsersCSV)
	if err != nil {
		t.Fatal(err)
	}
	data = bytes.ReplaceAll(data, []byte(f.prefix+"-9@gmail.com"), []byte("invalid-email"))
	if err = os.WriteFile(f.opts.UsersCSV, data, 0600); err != nil {
		t.Fatal(err)
	}
	for _, dry := range []bool{true, false} {
		f.opts.DryRun = dry
		if _, err = Owners(
			ctx,
			f.repo,
			f.opts,
			&bytes.Buffer{},
		); err == nil {
			t.Fatal("accepted invalid email")
		}
		if ownerSnapshot(t, f.db) != before {
			t.Fatal("invalid source left partial changes")
		}
	}
	for _, opts := range []OwnersRollbackOptions{{}, {All: true, Batch: "bad"}, {Batch: "bad"}} {
		if _, err = RollbackOwners(ctx, f.repo, opts); err == nil {
			t.Fatalf("accepted invalid rollback %+v", opts)
		}
	}
}

func TestOwnersRollbackForceAndAll(t *testing.T) {
	f := newOwnersFixture(t)
	ctx := context.Background()
	applied, err := Owners(
		ctx,
		f.repo,
		f.opts,
		&bytes.Buffer{},
	)
	if err != nil {
		t.Fatal(err)
	}
	var account, user int64
	err = f.db.Pool.QueryRow(
		ctx,
		`SELECT account_id,user_id FROM legacy_owner_items WHERE batch_id=$1 AND legacy_user_id=$2`,
		applied.Batch,
		f.prefix+"-09",
	).Scan(&account, &user)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.db.Pool.Exec(
		ctx,
		`INSERT INTO profiles(account_id,user_id) VALUES($1,$2)`,
		account,
		user,
	); err != nil {
		t.Fatal(err)
	}
	result, err := RollbackOwners(ctx, f.repo, OwnersRollbackOptions{Batch: applied.Batch, Force: true})
	if err != nil || len(result.RetainedAccounts) != 0 {
		t.Fatalf("force %+v %v", result, err)
	}
	var exists bool
	if err = f.db.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM accounts WHERE id=$1)`, account).Scan(&exists); err != nil || exists {
		t.Fatal("force retained account", err)
	}
	// A later assignment gets a distinct batch, with older rolled-back batches left alone.
	applied, err = Owners(
		ctx,
		f.repo,
		f.opts,
		&bytes.Buffer{},
	)
	if err != nil {
		t.Fatal(err)
	}
	result, err = RollbackOwners(ctx, f.repo, OwnersRollbackOptions{All: true})
	if err != nil || result.Batches != 1 {
		t.Fatalf("all %+v %v", result, err)
	}
	result, err = RollbackOwners(ctx, f.repo, OwnersRollbackOptions{All: true})
	if err != nil || result.Batches != 0 {
		t.Fatalf("all repeated %+v %v", result, err)
	}
}

func TestOwnersPersonalSQLReuseAndDuplicateLogin(t *testing.T) {
	f := newOwnersFixture(t)
	ctx := context.Background()
	// Simulate a company-only existing user, as owners legitimately creates for company members.
	if _, err := f.db.Pool.Exec(ctx, `DELETE FROM accounts WHERE id=$1`, f.existing.AccountID); err != nil {
		t.Fatal(err)
	}
	before := ownerSnapshot(t, f.db)
	applied, err := Owners(
		ctx,
		f.repo,
		f.opts,
		&bytes.Buffer{},
	)
	if err != nil {
		t.Fatal(err)
	}
	var correct bool
	err = f.db.Pool.QueryRow(ctx, `SELECT i.user_id=$2 AND NOT i.created_user AND i.created_account AND i.created_membership
  AND a.slug='u-' || u.uuid::text AND m.role='owner'
  FROM legacy_owner_items i JOIN accounts a ON a.id=i.account_id JOIN users u ON u.id=i.user_id
  JOIN account_members m ON m.id=i.membership_id WHERE i.batch_id=$1 AND i.legacy_user_id=$3`, applied.Batch, f.existing.ID, f.prefix+"-08").Scan(&correct)
	if err != nil || !correct {
		t.Fatal("personal SQL reuse", err)
	}
	if _, err = RollbackOwners(ctx, f.repo, OwnersRollbackOptions{Batch: applied.Batch}); err != nil {
		t.Fatal(err)
	}
	if ownerSnapshot(t, f.db) != before {
		t.Fatal("reused user with new personal account not restored")
	}
	// Two source users in a company share a login; the second profile selects the owner.
	data, err := os.ReadFile(f.opts.UsersCSV)
	if err != nil {
		t.Fatal(err)
	}
	data = bytes.ReplaceAll(data, []byte(f.prefix+"-2@gmail.com"), []byte(f.prefix+"-1@gmail.com"))
	if err = os.WriteFile(f.opts.UsersCSV, data, 0600); err != nil {
		t.Fatal(err)
	}
	applied, err = Owners(
		ctx,
		f.repo,
		f.opts,
		&bytes.Buffer{},
	)
	if err != nil {
		t.Fatal(err)
	}
	var ownerCount int
	err = f.db.Pool.QueryRow(ctx, `SELECT count(*) FROM account_members WHERE role='owner' AND account_id=(
  SELECT account_id FROM legacy_owner_items WHERE batch_id=$1 AND legacy_user_id=$2)`, applied.Batch, f.prefix+"-02").Scan(&ownerCount)
	if err != nil || ownerCount != 1 {
		t.Fatalf("duplicate-login owners=%d: %v", ownerCount, err)
	}
	if _, err = RollbackOwners(ctx, f.repo, OwnersRollbackOptions{Batch: applied.Batch}); err != nil {
		t.Fatal(err)
	}
	if ownerSnapshot(t, f.db) != before {
		t.Fatal("duplicate-login cycle not restored")
	}
}

func TestOwnersOnlyAssignsNewHoldingProfiles(t *testing.T) {
	f := newOwnersFixture(t)
	ctx := context.Background()
	first, err := Owners(ctx, f.repo, f.opts, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	before := ownerSnapshot(t, f.db)
	// A fresh source user arrives after the first owners batch.
	source, err := os.OpenFile(f.opts.UsersCSV, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	w := csv.NewWriter(source)
	id := f.prefix + "-11"
	err = w.Write([]string{id, id, id, id + "@gmail.com", "New Person", "", "{}", "1", "0", "", "", ""})
	if err != nil {
		t.Fatal(err)
	}
	w.Flush()
	if err = w.Error(); err != nil {
		t.Fatal(err)
	}
	if err = source.Close(); err != nil {
		t.Fatal(err)
	}
	synced, err := Sync(ctx, f.repo, f.sync, &bytes.Buffer{})
	if err != nil || synced.Creates != 1 || synced.Skips != 10 {
		t.Fatalf("new sync %+v %v", synced, err)
	}
	next, err := Owners(ctx, f.repo, f.opts, &bytes.Buffer{})
	if err != nil || next.Assigned != 1 || next.Individuals != 1 || next.Companies != 0 || next.Batch == first.Batch {
		t.Fatalf("new owners %+v %v", next, err)
	}
	if _, err = RollbackOwners(ctx, f.repo, OwnersRollbackOptions{Batch: next.Batch}); err != nil {
		t.Fatal(err)
	}
	// The original journaled destinations are unchanged by the incremental batch.
	var changed int
	err = f.db.Pool.QueryRow(ctx, `SELECT count(*) FROM legacy_owner_items i JOIN profiles p ON p.id=i.profile_id
  WHERE i.batch_id=$1 AND i.to_account_id<>p.account_id`, first.Batch).Scan(&changed)
	if err != nil || changed != 0 || before == ownerSnapshot(t, f.db) {
		t.Fatalf("incremental ownership %d %v", changed, err)
	}
	if _, err = RollbackOwners(ctx, f.repo, OwnersRollbackOptions{Batch: first.Batch}); err != nil {
		t.Fatal(err)
	}
}

func TestOwnersAcceptHoldingWithStaffMembers(t *testing.T) {
	f := newOwnersFixture(t)
	ctx := context.Background()
	// Platform staff join the holding account to review imports; that must not block assignment.
	tag, err := f.db.Pool.Exec(ctx, `WITH u AS (
  INSERT INTO users(email,password_hash,name,surname,status) VALUES($1,'x','Staff','Member','active') RETURNING id)
INSERT INTO account_members(account_id,user_id,role)
SELECT a.id,u.id,'owner' FROM accounts a, u WHERE a.uuid=$2`, f.prefix+"-staff@example.test", HoldingUUID)
	if err != nil || tag.RowsAffected() != 1 {
		t.Fatalf("staff membership rows=%d err=%v", tag.RowsAffected(), err)
	}
	f.opts.DryRun = true
	planned, err := Owners(ctx, f.repo, f.opts, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if planned.Individuals+planned.CompanyUsers == 0 {
		t.Fatalf("nothing planned with staff in holding: %+v", planned)
	}
}
