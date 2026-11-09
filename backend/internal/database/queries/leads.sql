-- Lead capture (TEC-552) and contact exchange (TEC-567). App reads are always scoped by account_id
-- and by the viewer (TEC-940): owners and admins of the account see every lead, any other member
-- only leads that came to their own employee profile (company_member_profiles) or to a card that
-- points at it. The role is read live from account_members, so an unknown viewer sees nothing.
-- The viewer condition is repeated in each read because sqlc cannot share it.

-- account-lint: cross-account public lead form: resolves a public route by path
-- name: GetLeadCaptureRouteByPath :one
-- The route a visitor submits through, with what decides whether its form is on. A card route
-- follows its card's current profile.
SELECT r.id, r.route_type, r.status, r.account_id::bigint AS account_id, r.presentation_config,
       r.card_id,
       COALESCE(c.profile_id, r.profile_id) AS profile_id,
       p.uuid AS profile_uuid,
       COALESCE(c.lead_capture_enabled, false)::boolean AS card_lead_capture_enabled,
       COALESCE(c.is_active, true)::boolean AS card_is_active,
       COALESCE(c.is_locked, false)::boolean AS card_is_locked
FROM routes r
JOIN accounts a ON a.id = r.account_id AND a.status = 'active'
LEFT JOIN nfc_cards c ON c.id = r.card_id AND c.deleted_at IS NULL
LEFT JOIN profiles p ON p.id = COALESCE(c.profile_id, r.profile_id) AND p.deleted_at IS NULL
WHERE lower(trim(r.path)) = lower(trim(sqlc.arg(path)::text))
  AND r.deleted_at IS NULL;

-- name: CreateLead :one
INSERT INTO leads (
    account_id, route_id, profile_id, card_id, source_type, route_path, kind,
    name, email, phone, message, company, job_title, sender_user_id, sender_profile_id,
    locale, consent_at, consent_version, ip_address, user_agent, locked
) VALUES (
    sqlc.arg(account_id), sqlc.narg(route_id), sqlc.narg(profile_id), sqlc.narg(card_id),
    sqlc.arg(source_type), sqlc.arg(route_path), sqlc.arg(kind), sqlc.arg(name), sqlc.arg(email),
    sqlc.arg(phone), sqlc.arg(message), sqlc.arg(company), sqlc.arg(job_title),
    sqlc.narg(sender_user_id), sqlc.narg(sender_profile_id), sqlc.arg(locale),
    sqlc.arg(consent_at), sqlc.arg(consent_version), sqlc.narg(ip_address), sqlc.arg(user_agent),
    sqlc.arg(locked)
)
RETURNING uuid, created_at;

-- account-lint: cross-account sender_username shows the public canonical route of the sender's profile (another account, exchange)
-- name: ListAccountLeads :many
-- A locked lead (TEC-941) matches q only on company: its name, email and phone are masked.
SELECT l.uuid, l.source_type, l.route_path, l.kind, l.name, l.email, l.phone, l.message,
       l.company, l.job_title, l.locale, l.consent_at, l.consent_version, l.read_at, l.created_at, l.locked,
       p.uuid AS profile_uuid, c.uuid AS card_uuid, sp.uuid AS sender_profile_uuid,
       sr.path AS sender_username
FROM leads l
LEFT JOIN profiles p ON p.id = l.profile_id
LEFT JOIN nfc_cards c ON c.id = l.card_id
LEFT JOIN profiles sp ON sp.id = l.sender_profile_id AND sp.deleted_at IS NULL
LEFT JOIN routes sr ON sr.profile_id = sp.id AND sr.route_type = 'canonical_profile'
    AND sr.status = 'active' AND sr.deleted_at IS NULL
WHERE l.account_id = sqlc.arg(account_id)
  AND (sqlc.narg(q)::text IS NULL
       OR (NOT l.locked AND (l.name ILIKE '%' || sqlc.narg(q)::text || '%'
                             OR l.email ILIKE '%' || sqlc.narg(q)::text || '%'
                             OR l.phone ILIKE '%' || sqlc.narg(q)::text || '%'))
       OR l.company ILIKE '%' || sqlc.narg(q)::text || '%')
  AND (sqlc.narg(source_type)::text IS NULL OR l.source_type = sqlc.narg(source_type)::text)
  AND (sqlc.narg(kind)::text IS NULL OR l.kind = sqlc.narg(kind)::text)
  AND (sqlc.narg(unread)::boolean IS NULL OR (l.read_at IS NULL) = sqlc.narg(unread)::boolean)
  AND (EXISTS (SELECT 1 FROM account_members vm
               WHERE vm.account_id = l.account_id AND vm.user_id = sqlc.arg(viewer_user_id)::bigint
                 AND vm.role IN ('owner', 'admin'))
       OR EXISTS (SELECT 1 FROM company_member_profiles ml
                  JOIN account_members vm ON vm.account_id = ml.account_id AND vm.id = ml.member_id
                  LEFT JOIN nfc_cards lc ON lc.id = l.card_id
                  WHERE ml.account_id = l.account_id AND vm.user_id = sqlc.arg(viewer_user_id)::bigint
                    AND (ml.profile_id = l.profile_id OR ml.profile_id = lc.profile_id)))
ORDER BY l.created_at DESC, l.id DESC
LIMIT sqlc.arg(limit_count) OFFSET sqlc.arg(offset_count);

-- name: CountAccountLeads :one
SELECT COUNT(*)::bigint
FROM leads l
WHERE l.account_id = sqlc.arg(account_id)
  AND (sqlc.narg(q)::text IS NULL
       OR (NOT l.locked AND (l.name ILIKE '%' || sqlc.narg(q)::text || '%'
                             OR l.email ILIKE '%' || sqlc.narg(q)::text || '%'
                             OR l.phone ILIKE '%' || sqlc.narg(q)::text || '%'))
       OR l.company ILIKE '%' || sqlc.narg(q)::text || '%')
  AND (sqlc.narg(source_type)::text IS NULL OR l.source_type = sqlc.narg(source_type)::text)
  AND (sqlc.narg(kind)::text IS NULL OR l.kind = sqlc.narg(kind)::text)
  AND (sqlc.narg(unread)::boolean IS NULL OR (l.read_at IS NULL) = sqlc.narg(unread)::boolean)
  AND (EXISTS (SELECT 1 FROM account_members vm
               WHERE vm.account_id = l.account_id AND vm.user_id = sqlc.arg(viewer_user_id)::bigint
                 AND vm.role IN ('owner', 'admin'))
       OR EXISTS (SELECT 1 FROM company_member_profiles ml
                  JOIN account_members vm ON vm.account_id = ml.account_id AND vm.id = ml.member_id
                  LEFT JOIN nfc_cards lc ON lc.id = l.card_id
                  WHERE ml.account_id = l.account_id AND vm.user_id = sqlc.arg(viewer_user_id)::bigint
                    AND (ml.profile_id = l.profile_id OR ml.profile_id = lc.profile_id)));

-- account-lint: cross-account sender_username shows the public canonical route of the sender's profile (another account, exchange)
-- name: GetAccountLead :one
SELECT l.uuid, l.source_type, l.route_path, l.kind, l.name, l.email, l.phone, l.message,
       l.company, l.job_title, l.locale, l.consent_at, l.consent_version, l.read_at, l.created_at, l.locked,
       p.uuid AS profile_uuid, c.uuid AS card_uuid, sp.uuid AS sender_profile_uuid,
       sr.path AS sender_username
FROM leads l
LEFT JOIN profiles p ON p.id = l.profile_id
LEFT JOIN nfc_cards c ON c.id = l.card_id
LEFT JOIN profiles sp ON sp.id = l.sender_profile_id AND sp.deleted_at IS NULL
LEFT JOIN routes sr ON sr.profile_id = sp.id AND sr.route_type = 'canonical_profile'
    AND sr.status = 'active' AND sr.deleted_at IS NULL
WHERE l.uuid = sqlc.arg(uuid) AND l.account_id = sqlc.arg(account_id)
  AND (EXISTS (SELECT 1 FROM account_members vm
               WHERE vm.account_id = l.account_id AND vm.user_id = sqlc.arg(viewer_user_id)::bigint
                 AND vm.role IN ('owner', 'admin'))
       OR EXISTS (SELECT 1 FROM company_member_profiles ml
                  JOIN account_members vm ON vm.account_id = ml.account_id AND vm.id = ml.member_id
                  LEFT JOIN nfc_cards lc ON lc.id = l.card_id
                  WHERE ml.account_id = l.account_id AND vm.user_id = sqlc.arg(viewer_user_id)::bigint
                    AND (ml.profile_id = l.profile_id OR ml.profile_id = lc.profile_id)));

-- name: MarkAccountLeadRead :execrows
UPDATE leads SET read_at = COALESCE(read_at, NOW())
WHERE uuid = sqlc.arg(uuid) AND account_id = sqlc.arg(account_id);

-- name: DeleteAccountLead :execrows
DELETE FROM leads WHERE uuid = sqlc.arg(uuid) AND account_id = sqlc.arg(account_id);

-- account-lint: cross-account visitor "delete my data" link: the signed token is checked before
-- name: DeleteVisitorExchange :one
-- KVKK "delete my data" link of the visitor copy email (TEC-940). Only exchanges; the token is
-- checked before. Deleting the row is what makes the link single use.
DELETE FROM leads WHERE uuid = sqlc.arg(uuid) AND kind = 'exchange'
RETURNING id;

-- name: DeleteExchangeCopyNotifications :execrows
-- The visitor copy email holds the visitor's details too; it goes with the exchange.
DELETE FROM notifications
WHERE source_event LIKE 'leads.exchange_copy:%' AND source_event = sqlc.arg(source_event)
  AND user_id IS NULL;

-- name: ListLeadNotifyUserIDs :many
-- Owners and admins of an account hear about new leads.
SELECT m.user_id
FROM account_members m
JOIN users u ON u.id = m.user_id AND u.deleted_at IS NULL
WHERE m.account_id = sqlc.arg(account_id) AND m.role IN ('owner', 'admin')
ORDER BY m.user_id;

-- name: GetExchangeSenderProfile :one
-- A signed-in visitor's own profile (TEC-567), scoped to their active account: the details an
-- exchange copies. Translation: the primary locale first; contacts: primary first, then order.
SELECT p.id, p.user_id,
       COALESCE(t.first_name, '')::text AS first_name,
       COALESCE(t.last_name, '')::text AS last_name,
       COALESCE(t.company_name, '')::text AS company_name,
       COALESCE(t.title, '')::text AS title,
       COALESCE((SELECT d.value FROM profile_contact_details d
                 WHERE d.profile_id = p.id AND d.category = 'email'
                 ORDER BY d.is_primary DESC, d.display_order, d.id LIMIT 1), '')::text AS email,
       COALESCE((SELECT d.value FROM profile_contact_details d
                 WHERE d.profile_id = p.id AND d.category IN ('phone', 'whatsapp')
                 ORDER BY (d.category = 'phone') DESC, d.is_primary DESC, d.display_order, d.id
                 LIMIT 1), '')::text AS phone
FROM profiles p
LEFT JOIN LATERAL (
    SELECT tr.first_name, tr.last_name, tr.company_name, tr.title
    FROM profile_translations tr
    WHERE tr.profile_id = p.id
    ORDER BY (tr.locale = p.primary_locale) DESC, tr.is_default DESC, tr.id
    LIMIT 1
) t ON true
WHERE p.uuid = sqlc.arg(uuid) AND p.account_id = sqlc.arg(account_id) AND p.deleted_at IS NULL;

-- account-lint: cross-account visitor copy email: profile id comes from the stored lead
-- name: GetLeadProfileName :one
-- Display name of the profile an exchange went to, for the visitor copy email (TEC-940).
SELECT trim(COALESCE(t.first_name, '') || ' ' || COALESCE(t.last_name, ''))::text AS name
FROM profiles p
LEFT JOIN LATERAL (
    SELECT tr.first_name, tr.last_name
    FROM profile_translations tr
    WHERE tr.profile_id = p.id
    ORDER BY (tr.locale = sqlc.arg(locale)::text) DESC, (tr.locale = p.primary_locale) DESC,
             tr.is_default DESC, tr.id
    LIMIT 1
) t ON true
WHERE p.id = sqlc.arg(id) AND p.deleted_at IS NULL;

-- Locked leads (TEC-941): received over the monthly quota. Unlocking is one-way.

-- name: HasLockedLeads :one
SELECT EXISTS (SELECT 1 FROM leads WHERE account_id = sqlc.arg(account_id) AND locked)::boolean;

-- name: UnlockLeadsBefore :execrows
-- A new month unlocks every lead locked in an earlier month.
UPDATE leads SET locked = false
WHERE account_id = sqlc.arg(account_id) AND locked AND created_at < sqlc.arg(month_start)::timestamptz;

-- name: UnlockMonthLeads :execrows
-- A plan upgrade unlocks the month's oldest locked leads of a kind, up to the new allowance.
UPDATE leads SET locked = false
WHERE leads.account_id = sqlc.arg(account_id) AND leads.locked AND leads.id IN (
    SELECT x.id FROM leads x
    WHERE x.account_id = sqlc.arg(account_id) AND x.kind = sqlc.arg(kind) AND x.locked
      AND x.created_at >= sqlc.arg(month_start)::timestamptz
    ORDER BY x.created_at, x.id
    LIMIT sqlc.arg(max_rows)::int
);
