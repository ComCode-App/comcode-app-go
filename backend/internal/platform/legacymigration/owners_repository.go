package legacymigration

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/password"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type ownerChange struct {
	Table  string `json:"table"`
	PK     int64  `json:"pk"`
	Column string `json:"column"`
	Old    *int64 `json:"old"`
	New    *int64 `json:"new"`
}

type ownerGroup struct {
	Batch, Key string
	Holding    int64
	IDs        []int
	Users      []ownerCandidate
}

type ownerItem struct {
	ID                                             int64
	LegacyID                                       string
	Profile, From, To, User, Account, Membership   int64
	CreatedUser, CreatedAccount, CreatedMembership bool
	Changes                                        []ownerChange
}

func assignOwnerGroup(ctx context.Context, tx pgx.Tx, group ownerGroup) error {
	company := len(group.IDs) >= 2
	var account int64
	if company {
		name := strings.TrimPrefix(strings.TrimPrefix(group.Key, "b:"), "override:")
		counts := map[string]int{}
		most := 0
		for _, i := range group.IDs {
			org := group.Users[i].Org
			if org != "" {
				counts[org]++
			}
		}
		for _, i := range group.IDs {
			org := group.Users[i].Org
			if counts[org] > most {
				name = org
				most = counts[org]
			}
		}
		if err := tx.QueryRow(
			ctx,
			`INSERT INTO accounts(type,name,slug) VALUES('company',$1,$2) RETURNING id`,
			short(name, 200),
			"legacy-"+uuid.NewString(),
		).Scan(&account); err != nil {
			return err
		}
	}
	ownerIndex := companyOwner(group.IDs, group.Users)
	for pos, i := range group.IDs {
		c := group.Users[i]
		item := ownerItem{
			LegacyID: c.ID,
			Profile:  c.ProfileID,
			From:     group.Holding,
			Account:  account,
			Changes:  []ownerChange{},
		}
		var matches int
		err := tx.QueryRow(ctx, `SELECT id,(SELECT count(*) FROM users WHERE lower(email)=$1 AND deleted_at IS NULL)
  FROM users WHERE lower(email)=$1 AND deleted_at IS NULL FOR UPDATE`, c.Email).Scan(&item.User, &matches)
		if err == nil && matches != 1 {
			return fmt.Errorf("ambiguous existing email for legacy user %s", c.ID)
		}
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if errors.Is(err, pgx.ErrNoRows) {
			secret := make([]byte, 32)
			if _, err = rand.Read(secret); err != nil {
				return err
			}
			hash, hashErr := password.Hash("Aa1" + base64.RawStdEncoding.EncodeToString(secret))
			clear(secret)
			if hashErr != nil {
				return hashErr
			}
			err = tx.QueryRow(
				ctx,
				`INSERT INTO users(email,name,surname,status,password_hash,password_set) VALUES($1,$2,$3,'pending',$4,FALSE) RETURNING id`,
				c.Email,
				short(c.Name, 100),
				short(c.Surname, 100),
				hash,
			).Scan(&item.User)
			if err != nil {
				return err
			}
			item.CreatedUser = true
		}
		if !company {
			if err = ownerPersonalAccount(ctx, tx, &item); err != nil {
				return err
			}
		} else {
			// A fresh company member must not retain the trigger's empty personal account.
			// Never remove a reused user's account.
			if item.CreatedUser {
				if _, err = tx.Exec(ctx, `DELETE FROM accounts WHERE personal_user_id=$1`, item.User); err != nil {
					return err
				}
			}
			item.CreatedAccount = pos == 0
			role := "member"
			if i == ownerIndex {
				role = "owner"
			}
			err = tx.QueryRow(
				ctx,
				`INSERT INTO account_members(account_id,user_id,role) VALUES($1,$2,$3)
                ON CONFLICT(account_id,user_id) DO NOTHING RETURNING id`,
				item.Account,
				item.User,
				role,
			).Scan(&item.Membership)
			item.CreatedMembership = err == nil
			if errors.Is(err, pgx.ErrNoRows) {
				err = tx.QueryRow(
					ctx,
					`SELECT id FROM account_members WHERE account_id=$1 AND user_id=$2`,
					item.Account,
					item.User,
				).Scan(&item.Membership)
			}
			if err != nil {
				return err
			}
			// Multiple legacy profiles may share one login. Promote that single membership
			// if the selected owner is a later profile belonging to the same user.
			if role == "owner" && !item.CreatedMembership {
				if _, err = tx.Exec(ctx, `UPDATE account_members SET role='owner' WHERE id=$1`, item.Membership); err != nil {
					return err
				}
			}
		}
		item.To = item.Account
		if err = moveOwnerRows(ctx, tx, &item); err != nil {
			return err
		}
		changes, err := json.Marshal(item.Changes)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `INSERT INTO legacy_owner_items(
   batch_id,legacy_user_id,profile_id,from_account_id,to_account_id,user_id,created_user,
   account_id,created_account,membership_id,created_membership,changes)
   VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
			group.Batch, item.LegacyID, item.Profile, item.From, item.To, item.User, item.CreatedUser,
			item.Account, item.CreatedAccount, item.Membership, item.CreatedMembership, changes)
		if err != nil {
			return err
		}
	}
	return nil
}

// All identifiers and predicates are internal constants. No CSV identifiers enter SQL.
func moveOwnerRows(ctx context.Context, tx pgx.Tx, item *ownerItem) error {
	targets := []struct {
		table, predicate, creator string
		arg                       any
	}{
		{table: "profiles", predicate: "id=$1", creator: "user_id", arg: item.Profile},
		{table: "routes", predicate: "profile_id=$1", creator: "owner_user_id", arg: item.Profile},
		{
			table:     "user_bank_accounts",
			predicate: "id IN (SELECT bank_account_id FROM profile_bank_accounts WHERE profile_id=$1)",
			creator:   "user_id",
			arg:       item.Profile,
		},
		{
			table:     "backgrounds",
			predicate: "uuid=$1",
			creator:   "created_by",
			arg:       syncBackgroundUUID(item.LegacyID),
		},
	}
	for _, target := range targets {
		cols := []string{"account_id"}
		if target.creator != "" {
			cols = append(cols, target.creator)
		}
		for _, col := range cols {
			predicate := target.predicate + " AND account_id=$2"
			// Account moves precede creator repair, so creators are now scoped to the destination.
			scope := item.From
			if col != "account_id" {
				scope = item.To
				predicate += " AND (" + col + " IS NULL OR " + col + " IN (SELECT user_id FROM account_members WHERE account_id=$3))"
			}
			query := `SELECT id,` + col + ` FROM ` + target.table + ` WHERE ` + predicate + ` FOR UPDATE`
			args := []any{target.arg, scope}
			if col != "account_id" {
				args = append(args, item.From)
			}
			rows, err := tx.Query(ctx, query, args...)
			if err != nil {
				return err
			}
			changes := []ownerChange{}
			for rows.Next() {
				ch := ownerChange{Table: target.table, Column: col, New: &item.To}
				if col != "account_id" {
					ch.New = &item.User
				}
				if err = rows.Scan(&ch.PK, &ch.Old); err != nil {
					rows.Close()
					return err
				}
				changes = append(changes, ch)
			}
			err = rows.Err()
			rows.Close()
			if err != nil {
				return err
			}
			for _, ch := range changes {
				if _, err = tx.Exec(
					ctx,
					`UPDATE `+ch.Table+` SET `+col+`=$2 WHERE id=$1`,
					ch.PK,
					ch.New,
				); err != nil {
					return err
				}
				item.Changes = append(item.Changes, ch)
			}
		}
	}
	return nil
}

func validOwnerChange(c ownerChange) bool {
	switch c.Table {
	case "profiles", "user_bank_accounts":
		return c.Column == "account_id" || c.Column == "user_id"
	case "routes":
		return c.Column == "account_id" || c.Column == "owner_user_id"
	case "backgrounds":
		return c.Column == "account_id" || c.Column == "created_by"
	}
	return false
}

// accountDataTables derives the safety check from the live schema, including future account tables.
func accountDataTables(ctx context.Context, tx pgx.Tx) ([]string, error) {
	rows, err := tx.Query(ctx, `SELECT table_name FROM information_schema.columns
  WHERE table_schema='public' AND column_name='account_id'
  AND table_name NOT IN ('account_members','legacy_owner_items') ORDER BY table_name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	tables := []string{}
	for rows.Next() {
		var s string
		if err = rows.Scan(&s); err != nil {
			return nil, err
		}
		tables = append(tables, s)
	}
	return tables, rows.Err()
}

func hasAccountData(ctx context.Context, tx pgx.Tx, tables []string, id int64) (bool, error) {
	for _, table := range tables {
		var exists bool
		query := `SELECT EXISTS(SELECT 1 FROM ` + pgx.Identifier{table}.Sanitize() + ` WHERE account_id=$1)`
		if err := tx.QueryRow(ctx, query, id).Scan(&exists); err != nil {
			return false, err
		}
		if exists {
			return true, nil
		}
	}
	return false, nil
}

func ownerConflict(ch ownerChange) string {
	return fmt.Sprintf(
		"%s:%d.%s",
		ch.Table,
		ch.PK,
		ch.Column,
	)
}

func ownerBatchIDs(ctx context.Context, tx pgx.Tx, batch string, all bool) ([]string, error) {
	rows, err := tx.Query(
		ctx,
		`SELECT id::text FROM legacy_owner_batches WHERE ($1 OR id=$2::uuid) ORDER BY created_at DESC,id DESC FOR UPDATE`,
		all,
		nullableBatch(batch),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if !all && len(ids) == 0 {
		return nil, errors.New("unknown owner batch")
	}
	return ids, rows.Err()
}
func nullableBatch(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}

// ownerPersonalAccount reuses the signup trigger's SQL from migration 000033, including
// its UUID-based unique slug, for an existing company-only user who needs a personal account.
func ownerPersonalAccount(ctx context.Context, tx pgx.Tx, item *ownerItem) error {
	err := tx.QueryRow(ctx, `SELECT id FROM accounts WHERE personal_user_id=$1`, item.User).Scan(&item.Account)
	item.CreatedAccount = item.CreatedUser
	if errors.Is(err, pgx.ErrNoRows) {
		err = tx.QueryRow(ctx, `INSERT INTO accounts(type,name,slug,personal_user_id)
   SELECT 'personal',COALESCE(NULLIF(TRIM(name || ' ' || surname), ''),email),'u-' || uuid::text,id
   FROM users WHERE id=$1 RETURNING id`, item.User).Scan(&item.Account)
		item.CreatedAccount = err == nil
	}
	if err != nil {
		return err
	}
	err = tx.QueryRow(ctx, `SELECT id FROM account_members WHERE account_id=$1 AND user_id=$2`,
		item.Account, item.User).Scan(&item.Membership)
	item.CreatedMembership = item.CreatedUser
	if errors.Is(err, pgx.ErrNoRows) {
		err = tx.QueryRow(ctx, `INSERT INTO account_members(account_id,user_id,role)
   VALUES($1,$2,'owner') RETURNING id`, item.Account, item.User).Scan(&item.Membership)
		item.CreatedMembership = err == nil
	}
	return err
}
