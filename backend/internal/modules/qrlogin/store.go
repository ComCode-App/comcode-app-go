package qrlogin

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// Session states, in order: pending → claimed → approved | rejected → exchanged. "exchanged" is
// not stored: a successful exchange deletes the key, so a replay finds nothing.
const (
	StatusPending  = "pending"
	StatusClaimed  = "claimed"
	StatusApproved = "approved"
	StatusRejected = "rejected"
)

var (
	// ErrNotFound covers unknown, expired, burned and already exchanged sessions.
	ErrNotFound = errors.New("qr login session not found")
	// ErrResolved means the session was already approved or rejected (or is not approved yet
	// when exchanging).
	ErrResolved = errors.New("qr login session already resolved")
	// ErrClaimed means another user claimed the session first.
	ErrClaimed = errors.New("qr login session claimed by another user")
	// ErrInvalidSecret covers a wrong browser secret or exchange token.
	ErrInvalidSecret = errors.New("qr login secret is invalid")
)

// Session is the Redis-held state of one QR sign-in attempt (one browser tab).
type Session struct {
	ChannelID  string
	Status     string
	SecretHash string
	UserAgent  string
	IP         string
	Location   Location
	CreatedAt  time.Time
	ExpiresAt  time.Time
	ClaimedBy  string
}

// Store keeps QR sessions in Redis only (short TTL, never Postgres). Every state transition is
// one Lua script, so concurrent claim / approve / reject / exchange calls cannot both win.
type Store struct {
	rdb *redis.Client
	env string
}

// NewStore creates a Redis-backed store.
func NewStore(rdb *redis.Client, appEnv string) *Store {
	if appEnv == "" {
		appEnv = "dev"
	}
	return &Store{rdb: rdb, env: appEnv}
}

// key never contains the raw session id: the id is in the QR and works like a bearer value.
func (s *Store) key(sessionID string) string {
	return fmt.Sprintf("app:%s:qrlogin:%s", s.env, hashSecret(sessionID))
}

func hashSecret(v string) string {
	sum := sha256.Sum256([]byte(v))
	return hex.EncodeToString(sum[:])
}

// Create stores a new pending session that expires with ttl.
func (s *Store) Create(ctx context.Context, sessionID string, sess Session, ttl time.Duration) error {
	key := s.key(sessionID)
	pipe := s.rdb.TxPipeline()
	pipe.HSet(ctx, key, map[string]any{
		"status":       StatusPending,
		"channel_id":   sess.ChannelID,
		"secret_hash":  sess.SecretHash,
		"ua":           sess.UserAgent,
		"ip":           sess.IP,
		"country_code": sess.Location.CountryCode,
		"country":      sess.Location.Country,
		"city":         sess.Location.City,
		"created_at":   sess.CreatedAt.UTC().Unix(),
		"expires_at":   sess.ExpiresAt.UTC().Unix(),
		"attempts":     0,
	})
	pipe.Expire(ctx, key, ttl)
	_, err := pipe.Exec(ctx)
	return err
}

// Get loads a session (ErrNotFound when missing or expired).
func (s *Store) Get(ctx context.Context, sessionID string) (Session, error) {
	vals, err := s.rdb.HGetAll(ctx, s.key(sessionID)).Result()
	if err != nil {
		return Session{}, err
	}
	if len(vals) == 0 || vals["status"] == "" {
		return Session{}, ErrNotFound
	}
	unix := func(k string) time.Time {
		n, _ := strconv.ParseInt(vals[k], 10, 64)
		if n == 0 {
			return time.Time{}
		}
		return time.Unix(n, 0).UTC()
	}
	return Session{
		ChannelID:  vals["channel_id"],
		Status:     vals["status"],
		SecretHash: vals["secret_hash"],
		UserAgent:  vals["ua"],
		IP:         vals["ip"],
		Location:   Location{CountryCode: vals["country_code"], Country: vals["country"], City: vals["city"]},
		CreatedAt:  unix("created_at"),
		ExpiresAt:  unix("expires_at"),
		ClaimedBy:  vals["claimed_by"],
	}, nil
}

// claimScript: the first signed-in viewer claims a pending session.
// Returns "missing" | "taken" (by someone else) | "first" | <current status>.
var claimScript = redis.NewScript(`
local st = redis.call('HGET', KEYS[1], 'status')
if not st then return 'missing' end
local by = redis.call('HGET', KEYS[1], 'claimed_by')
if by and by ~= '' and by ~= ARGV[1] then return 'taken' end
if st == 'pending' then
  redis.call('HSET', KEYS[1], 'status', 'claimed', 'claimed_by', ARGV[1])
  return 'first'
end
return st
`)

// Claim binds the session to the viewing user. first is true only for the first claim (the tab
// is told "scanned" once).
func (s *Store) Claim(ctx context.Context, sessionID, userUUID string) (first bool, err error) {
	res, err := claimScript.Run(ctx, s.rdb, []string{s.key(sessionID)}, userUUID).Text()
	if err != nil {
		return false, err
	}
	switch res {
	case "missing":
		return false, ErrNotFound
	case "taken":
		return false, ErrClaimed
	case "first":
		return true, nil
	default:
		return false, nil
	}
}

// resolveScript moves pending|claimed → approved|rejected exactly once and starts the exchange
// window. ARGV: user, new status, exchange token, ttl seconds, exchange token hash.
var resolveScript = redis.NewScript(`
local st = redis.call('HGET', KEYS[1], 'status')
if not st then return 'missing' end
local by = redis.call('HGET', KEYS[1], 'claimed_by')
if by and by ~= '' and by ~= ARGV[1] then return 'taken' end
if st ~= 'pending' and st ~= 'claimed' then return 'resolved' end
redis.call('HSET', KEYS[1], 'status', ARGV[2], 'claimed_by', ARGV[1],
  'exchange_token', ARGV[3], 'exchange_hash', ARGV[5])
redis.call('EXPIRE', KEYS[1], tonumber(ARGV[4]))
return 'ok'
`)

// Resolve approves or rejects the session for userUUID. exchangeToken is set for approvals only;
// ttl is the exchange window (it also bounds how long a rejection stays readable).
func (s *Store) Resolve(ctx context.Context, sessionID, userUUID, status, exchangeToken string, ttl time.Duration) error {
	secs := int64(ttl.Seconds())
	if secs < 1 {
		secs = 1
	}
	exchangeHash := ""
	if exchangeToken != "" {
		exchangeHash = hashSecret(exchangeToken)
	}
	res, err := resolveScript.Run(ctx, s.rdb, []string{s.key(sessionID)},
		userUUID, status, exchangeToken, secs, exchangeHash).Text()
	if err != nil {
		return err
	}
	switch res {
	case "ok":
		return nil
	case "missing":
		return ErrNotFound
	case "taken":
		return ErrClaimed
	default:
		return ErrResolved
	}
}

// stateScript returns the status (and the exchange token once approved) to the tab holding the
// browser secret. A wrong secret burns an attempt; the session is deleted at the limit.
// ARGV: secret hash, max attempts.
var stateScript = redis.NewScript(`
local st = redis.call('HGET', KEYS[1], 'status')
if not st then return {'missing'} end
if redis.call('HGET', KEYS[1], 'secret_hash') ~= ARGV[1] then
  local n = redis.call('HINCRBY', KEYS[1], 'attempts', 1)
  if n >= tonumber(ARGV[2]) then redis.call('DEL', KEYS[1]) end
  return {'invalid'}
end
return {st, redis.call('HGET', KEYS[1], 'exchange_token') or ''}
`)

// State reports the session status to the browser tab.
func (s *Store) State(ctx context.Context, sessionID, secretHash string, maxAttempts int) (status, exchangeToken string, err error) {
	res, err := stateScript.Run(ctx, s.rdb, []string{s.key(sessionID)}, secretHash, maxAttempts).StringSlice()
	if err != nil {
		return "", "", err
	}
	switch res[0] {
	case "missing":
		return "", "", ErrNotFound
	case "invalid":
		return "", "", ErrInvalidSecret
	}
	if len(res) > 1 {
		exchangeToken = res[1]
	}
	return res[0], exchangeToken, nil
}

// exchangeScript consumes an approved session: single use, the key is deleted. A wrong secret or
// token burns an attempt; the session is deleted at the limit.
// ARGV: secret hash, exchange token hash, max attempts.
var exchangeScript = redis.NewScript(`
local st = redis.call('HGET', KEYS[1], 'status')
if not st then return {'missing'} end
local sh = redis.call('HGET', KEYS[1], 'secret_hash')
local eh = redis.call('HGET', KEYS[1], 'exchange_hash') or ''
if sh ~= ARGV[1] or eh == '' or eh ~= ARGV[2] then
  local n = redis.call('HINCRBY', KEYS[1], 'attempts', 1)
  if n >= tonumber(ARGV[3]) then redis.call('DEL', KEYS[1]) end
  return {'invalid'}
end
if st ~= 'approved' then return {'state', st} end
local out = {'ok',
  redis.call('HGET', KEYS[1], 'claimed_by') or '',
  redis.call('HGET', KEYS[1], 'ua') or '',
  redis.call('HGET', KEYS[1], 'ip') or ''}
redis.call('DEL', KEYS[1])
return out
`)

// Consumed is the result of a successful exchange.
type Consumed struct {
	UserUUID  string
	UserAgent string
	IP        string
}

// Exchange atomically validates and deletes an approved session.
func (s *Store) Exchange(ctx context.Context, sessionID, secretHash, exchangeToken string, maxAttempts int) (Consumed, error) {
	res, err := exchangeScript.Run(ctx, s.rdb, []string{s.key(sessionID)},
		secretHash, hashSecret(exchangeToken), maxAttempts).StringSlice()
	if err != nil {
		return Consumed{}, err
	}
	switch res[0] {
	case "missing":
		return Consumed{}, ErrNotFound
	case "invalid":
		return Consumed{}, ErrInvalidSecret
	case "state":
		return Consumed{}, ErrResolved
	}
	if len(res) < 4 || res[1] == "" {
		return Consumed{}, ErrNotFound
	}
	return Consumed{UserUUID: res[1], UserAgent: res[2], IP: res[3]}, nil
}
