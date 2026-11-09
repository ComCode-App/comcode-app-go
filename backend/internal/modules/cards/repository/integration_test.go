package repository_test

// Postgres integration tests (TEC-536). Skipped unless TEST_DATABASE_URL is set; see
// internal/testutil/pgtest.

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/cards/repository"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/testutil/pgtest"
)

func TestMain(m *testing.M) { pgtest.Main(m) }

func raised(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "P0001"
}

func TestIntegrationProtectCardIdentityTrigger(t *testing.T) {
	d := pgtest.Open(t)
	ctx := context.Background()
	card := d.CreateCard(t, d.CreateUser(t))
	exec := func(stmt string, args ...any) error {
		_, err := d.Pool.Exec(ctx, stmt, append([]any{card.ID}, args...)...)
		return err
	}

	if err := exec(`UPDATE nfc_cards SET smart_code = 'cc-changed' WHERE id = $1`); !raised(err) {
		t.Fatalf("changing smart_code: %v, want trigger exception", err)
	}
	// card_uid can be recorded once...
	uid := "UID-" + pgtest.Unique()
	if err := exec(`UPDATE nfc_cards SET card_uid = $2 WHERE id = $1`, uid); err != nil {
		t.Fatalf("recording card_uid: %v", err)
	}
	// ...re-saved unchanged together with other fields...
	if err := exec(`UPDATE nfc_cards SET card_uid = $2, name = 'Renamed', is_locked = true WHERE id = $1`, uid); err != nil {
		t.Fatalf("saving unchanged identity: %v", err)
	}
	// ...but never changed or cleared afterwards.
	if err := exec(`UPDATE nfc_cards SET card_uid = 'UID-other' WHERE id = $1`); !raised(err) {
		t.Fatalf("changing card_uid: %v, want trigger exception", err)
	}
	if err := exec(`UPDATE nfc_cards SET card_uid = NULL WHERE id = $1`); !raised(err) {
		t.Fatalf("clearing card_uid: %v, want trigger exception", err)
	}
}

func TestIntegrationActivateCardByCode(t *testing.T) {
	d := pgtest.Open(t)
	ctx := context.Background()
	repo := repository.NewPostgres(d.Queries)
	factory, customer := d.CreateUser(t), d.CreateUser(t)
	card := d.CreateCard(t, factory)
	code := "ACT-" + pgtest.Unique()
	if _, err := d.Pool.Exec(ctx, `UPDATE nfc_cards SET activation_code = $2, is_active = false, status = 'unactivated' WHERE id = $1`, card.ID, code); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Pool.Exec(ctx, `INSERT INTO routes (path, route_type, status, is_immutable, card_id, account_id)
		VALUES ('c/' || $2::text, 'card_route', 'reserved', true, $1, $3)`, card.ID, card.SmartCode, factory.AccountID); err != nil {
		t.Fatal(err)
	}

	name := "My card"
	got, err := repo.ActivateCardByCode(ctx, code, customer.AccountID, customer.ID, &name, nil)
	if err != nil {
		t.Fatalf("activate: %v", err)
	}
	if !got.IsActive || got.Name != name || got.UUID != card.UUID || got.SmartCode != card.SmartCode {
		t.Fatalf("activated card = %+v", got)
	}

	var routeStatus string
	var routeAccount, cardAccount int64
	var activation *string
	if err := d.Pool.QueryRow(ctx, `SELECT r.status, r.account_id, c.account_id, c.activation_code
		FROM routes r JOIN nfc_cards c ON c.id = r.card_id WHERE c.id = $1`, card.ID).
		Scan(&routeStatus, &routeAccount, &cardAccount, &activation); err != nil {
		t.Fatal(err)
	}
	if routeStatus != "active" || routeAccount != customer.AccountID || cardAccount != customer.AccountID || activation != nil {
		t.Fatalf("after activation: route %s account %d, card account %d, code %v", routeStatus, routeAccount, cardAccount, activation)
	}

	// The code is single use.
	if _, err := repo.ActivateCardByCode(ctx, code, factory.AccountID, factory.ID, nil, nil); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("reusing an activation code: %v, want ErrNoRows", err)
	}
}
