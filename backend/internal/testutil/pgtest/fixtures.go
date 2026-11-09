package pgtest

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"testing"

	"github.com/google/uuid"
)

// User is a seeded user with the personal account the users trigger creates for it.
type User struct {
	ID        int64
	UUID      uuid.UUID
	AccountID int64
}

// Unique returns a short random suffix for values with unique constraints (emails, slugs).
func Unique() string {
	b := make([]byte, 5)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// CreateUser inserts an active user; trg_users_create_personal_account adds its personal
// account with the user as owner.
func (d *DB) CreateUser(t testing.TB) User {
	t.Helper()
	ctx := context.Background()
	var u User
	err := d.Pool.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, name, surname, status)
		VALUES ($1, 'x', 'Test', 'User', 'active')
		RETURNING id, uuid`, "user-"+Unique()+"@example.test").Scan(&u.ID, &u.UUID)
	if err != nil {
		t.Fatalf("pgtest: create user: %v", err)
	}
	if err := d.Pool.QueryRow(ctx, `SELECT id FROM accounts WHERE personal_user_id = $1`, u.ID).Scan(&u.AccountID); err != nil {
		t.Fatalf("pgtest: personal account: %v", err)
	}
	return u
}

// CreateProduct inserts an active physical card product and returns its uuid.
func (d *DB) CreateProduct(t testing.TB, basePrice float64) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	s := Unique()
	err := d.Pool.QueryRow(context.Background(), `
		INSERT INTO products (name, slug, status, base_price, sku)
		VALUES ($1, $2, 'active', $3, $4)
		RETURNING uuid`, "Card "+s, "card-"+s, basePrice, "SKU-"+s).Scan(&id)
	if err != nil {
		t.Fatalf("pgtest: create product: %v", err)
	}
	return id
}

// Card is a seeded NFC card.
type Card struct {
	ID        int64
	UUID      uuid.UUID
	SmartCode string
}

// CreateCard inserts an active, unlocked card of the user's personal account with a random
// smart code and no card_uid.
func (d *DB) CreateCard(t testing.TB, u User) Card {
	t.Helper()
	c := Card{SmartCode: "cc-" + Unique()}
	err := d.Pool.QueryRow(context.Background(), `
		INSERT INTO nfc_cards (user_id, account_id, name, smart_code)
		VALUES ($1, $2, 'Test card', $3)
		RETURNING id, uuid`, u.ID, u.AccountID, c.SmartCode).Scan(&c.ID, &c.UUID)
	if err != nil {
		t.Fatalf("pgtest: create card: %v", err)
	}
	return c
}
