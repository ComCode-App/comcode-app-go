package legacymigration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type OwnersRollbackOptions struct {
	Batch      string
	All, Force bool
}

type OwnersRollbackSummary struct {
	Batches          int      `json:"batches"`
	Restored         int      `json:"restored"`
	Conflicts        []string `json:"conflicts"`
	RetainedAccounts []int64  `json:"retained_accounts"`
	RetainedUsers    []int64  `json:"retained_users"`
}

// RollbackOwners restores journaled values conditionally and never deletes a conflicting row.
// Accounts with remaining data and their memberships/users are retained and reported.
func RollbackOwners(ctx context.Context, repo *Postgres, opts OwnersRollbackOptions) (OwnersRollbackSummary, error) {
	result := OwnersRollbackSummary{Conflicts: []string{}, RetainedAccounts: []int64{}, RetainedUsers: []int64{}}
	if opts.All == (opts.Batch != "") {
		return result, errors.New("specify exactly one of --batch or --all")
	}
	if opts.Batch != "" {
		if _, err := uuid.Parse(opts.Batch); err != nil {
			return result, errors.New("invalid batch UUID")
		}
	}
	tx, err := repo.pool.Begin(ctx)
	if err != nil {
		return result, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(560560)`); err != nil {
		return result, err
	}
	batches, err := ownerBatchIDs(
		ctx,
		tx,
		opts.Batch,
		opts.All,
	)
	if err != nil {
		return result, err
	}
	tables, err := accountDataTables(ctx, tx)
	if err != nil {
		return result, err
	}
	for _, batch := range batches {
		var mode string
		if err = tx.QueryRow(ctx, `SELECT mode FROM legacy_owner_batches WHERE id=$1`, batch).Scan(&mode); err != nil {
			return result, err
		}
		if mode == "rolled_back" {
			continue
		}
		items, loadErr := loadOwnerItems(ctx, tx, batch)
		if loadErr != nil {
			return result, loadErr
		}
		protected := map[int64]bool{}
		for _, item := range items {
			for i := len(item.Changes) - 1; i >= 0; i-- {
				ch := item.Changes[i]
				if !validOwnerChange(ch) {
					return result, errors.New("invalid journal identifier")
				}
				tag, updateErr := tx.Exec(
					ctx,
					`UPDATE `+ch.Table+` SET `+ch.Column+`=$2 WHERE id=$1 AND `+ch.Column+` IS NOT DISTINCT FROM $3::bigint`,
					ch.PK,
					ch.Old,
					ch.New,
				)
				if updateErr != nil {
					return result, updateErr
				}
				if tag.RowsAffected() == 1 {
					result.Restored++
					continue
				}
				// A missing or subsequently edited row is a conflict too; only the exact
				// recorded new value may be restored. Batch status makes retries a no-op.
				result.Conflicts = append(result.Conflicts, ownerConflict(ch))
				protected[item.Account] = true
			}
		}
		retained := map[int64]bool{}
		for _, item := range items {
			if !item.CreatedAccount {
				continue
			}
			// Parent locks also serialize FK-backed writes while cleanup is checked.
			var id int64
			lockErr := tx.QueryRow(ctx, `SELECT id FROM accounts WHERE id=$1 FOR UPDATE`, item.Account).Scan(&id)
			if errors.Is(lockErr, pgx.ErrNoRows) {
				continue
			}
			if lockErr != nil {
				return result, lockErr
			}
			data, dataErr := hasAccountData(
				ctx,
				tx,
				tables,
				item.Account,
			)
			if dataErr != nil {
				return result, dataErr
			}
			// New or changed memberships are also non-legacy data.
			var extraMembers bool
			err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM account_members m WHERE account_id=$1
    AND NOT EXISTS(SELECT 1 FROM legacy_owner_items i WHERE i.batch_id=$2
     AND i.membership_id=m.id AND i.created_membership AND i.user_id=m.user_id))`, item.Account, batch).Scan(&extraMembers)
			if err != nil {
				return result, err
			}
			if protected[item.Account] || ((data || extraMembers) && !opts.Force) {
				retained[item.Account] = true
				result.RetainedAccounts = append(result.RetainedAccounts, item.Account)
			}
		}
		for _, item := range items {
			if !item.CreatedMembership || retained[item.Account] {
				continue
			}
			if _, err = tx.Exec(
				ctx,
				`DELETE FROM account_members WHERE id=$1 AND account_id=$2 AND user_id=$3`,
				item.Membership,
				item.Account,
				item.User,
			); err != nil {
				return result, err
			}
		}
		for _, item := range items {
			if item.CreatedAccount && !retained[item.Account] {
				if _, err = tx.Exec(ctx, `DELETE FROM accounts WHERE id=$1`, item.Account); err != nil {
					return result, err
				}
			}
		}
		for _, item := range items {
			if !item.CreatedUser {
				continue
			}
			tag, deleteErr := tx.Exec(ctx, `DELETE FROM users WHERE id=$1 AND last_login_at IS NULL AND status='pending'
    AND NOT EXISTS(SELECT 1 FROM account_members WHERE user_id=$1)`, item.User)
			if deleteErr != nil {
				return result, deleteErr
			}
			if tag.RowsAffected() == 0 {
				result.RetainedUsers = append(result.RetainedUsers, item.User)
			}
		}
		result.Batches++
		if _, err = tx.Exec(
			ctx,
			`UPDATE legacy_owner_batches SET mode='rolled_back',
                summary=summary || jsonb_build_object('rollback',$2::jsonb) WHERE id=$1`,
			batch,
			mustRollbackJSON(result),
		); err != nil {
			return result, err
		}
	}
	return result, tx.Commit(ctx)
}

func mustRollbackJSON(summary OwnersRollbackSummary) []byte {
	// All fields are scalar values and slices, so encoding cannot fail.
	data, _ := json.Marshal(summary)
	return data
}

func loadOwnerItems(ctx context.Context, tx pgx.Tx, batch string) ([]ownerItem, error) {
	rows, err := tx.Query(ctx, `SELECT id,legacy_user_id,profile_id,from_account_id,to_account_id,user_id,created_user,
  account_id,created_account,membership_id,created_membership,changes FROM legacy_owner_items
  WHERE batch_id=$1 ORDER BY id DESC`, batch)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []ownerItem{}
	for rows.Next() {
		var item ownerItem
		var changes []byte
		err = rows.Scan(&item.ID, &item.LegacyID, &item.Profile, &item.From, &item.To, &item.User, &item.CreatedUser,
			&item.Account, &item.CreatedAccount, &item.Membership, &item.CreatedMembership, &changes)
		if err != nil {
			return nil, err
		}
		if err = json.Unmarshal(changes, &item.Changes); err != nil {
			return nil, fmt.Errorf("decode owner journal: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
