-- TEC-574: company analytics. Every query is scoped by account_id and by the caller's live role:
-- owner/admin see the whole company, office managers only profiles linked to members assigned to
-- one of their granted offices. Subjects (profiles, routes, cards) are resolved to the employee
-- profile they currently point at; counts come from event_aggregates_daily only (no raw scan).
-- all_scope (owner/admin without office/group filter) also counts subjects that point at no live
-- profile of the account (e.g. a smart link to a URL); those are returned with profile_id = 0.
-- The scope CTEs are repeated in each query because sqlc cannot share them.

-- name: CompanyAnalyticsActorRole :one
-- Non-locking read of the caller's role in a company account (analytics is read-only).
SELECT m.role
FROM account_members m
JOIN accounts a ON a.id = m.account_id AND a.type = 'company'
WHERE m.account_id = sqlc.arg(account_id) AND m.user_id = sqlc.arg(user_id);

-- name: CompanyAnalyticsByProfile :many
WITH sp AS (
    SELECT p.id
    FROM profiles p
    LEFT JOIN company_member_profiles l ON l.account_id = p.account_id AND l.profile_id = p.id
    LEFT JOIN company_member_assignments a ON a.account_id = l.account_id AND a.member_id = l.member_id
    LEFT JOIN company_offices off ON off.account_id = p.account_id AND off.id = a.office_id
    LEFT JOIN company_groups g ON g.account_id = p.account_id AND g.id = a.group_id
    WHERE p.account_id = sqlc.arg(account_id) AND p.deleted_at IS NULL
    AND (sqlc.arg(actor_role)::text IN ('owner', 'admin')
        OR (sqlc.arg(actor_role)::text = 'office_manager' AND EXISTS (
            SELECT 1 FROM account_member_scopes s JOIN account_members actor ON actor.id = s.member_id
            WHERE s.account_id = p.account_id AND actor.user_id = sqlc.arg(actor_id)::bigint
              AND s.office_id = a.office_id)))
    AND (sqlc.narg(office_uuid)::uuid IS NULL OR off.uuid = sqlc.narg(office_uuid)::uuid)
    AND (sqlc.narg(group_uuid)::uuid IS NULL OR g.uuid = sqlc.narg(group_uuid)::uuid)
), subj AS (
    SELECT 'profile'::text AS subject_type, p.uuid::text AS subject_id, p.id AS profile_id, ''::text AS route_type
    FROM profiles p WHERE p.account_id = sqlc.arg(account_id) AND p.deleted_at IS NULL
    UNION ALL
    SELECT 'route'::text, r.uuid::text, r.profile_id, r.route_type::text
    FROM routes r WHERE r.account_id = sqlc.arg(account_id) AND r.deleted_at IS NULL
    UNION ALL
    SELECT 'card'::text, c.uuid::text, c.profile_id, ''::text
    FROM nfc_cards c WHERE c.account_id = sqlc.arg(account_id) AND c.deleted_at IS NULL
), scoped AS (
    SELECT subj.subject_type, subj.subject_id, subj.route_type, sp.id AS profile_id
    FROM subj LEFT JOIN sp ON sp.id = subj.profile_id
    WHERE sp.id IS NOT NULL
       OR (sqlc.arg(all_scope)::boolean AND sqlc.arg(actor_role)::text IN ('owner', 'admin'))
)
SELECT COALESCE(scoped.profile_id, 0)::bigint AS profile_id, scoped.subject_type::text AS subject_type,
       scoped.route_type::text AS route_type, d.event_type, SUM(d.total_count)::bigint AS total
FROM scoped
JOIN event_aggregates_daily d ON d.subject_type = scoped.subject_type AND d.subject_id = scoped.subject_id
WHERE d.bucket_date BETWEEN sqlc.arg(from_date)::date AND sqlc.arg(to_date)::date
GROUP BY 1, 2, 3, 4;

-- name: CompanyAnalyticsDaily :many
WITH sp AS (
    SELECT p.id
    FROM profiles p
    LEFT JOIN company_member_profiles l ON l.account_id = p.account_id AND l.profile_id = p.id
    LEFT JOIN company_member_assignments a ON a.account_id = l.account_id AND a.member_id = l.member_id
    LEFT JOIN company_offices off ON off.account_id = p.account_id AND off.id = a.office_id
    LEFT JOIN company_groups g ON g.account_id = p.account_id AND g.id = a.group_id
    WHERE p.account_id = sqlc.arg(account_id) AND p.deleted_at IS NULL
    AND (sqlc.arg(actor_role)::text IN ('owner', 'admin')
        OR (sqlc.arg(actor_role)::text = 'office_manager' AND EXISTS (
            SELECT 1 FROM account_member_scopes s JOIN account_members actor ON actor.id = s.member_id
            WHERE s.account_id = p.account_id AND actor.user_id = sqlc.arg(actor_id)::bigint
              AND s.office_id = a.office_id)))
    AND (sqlc.narg(office_uuid)::uuid IS NULL OR off.uuid = sqlc.narg(office_uuid)::uuid)
    AND (sqlc.narg(group_uuid)::uuid IS NULL OR g.uuid = sqlc.narg(group_uuid)::uuid)
), subj AS (
    SELECT 'profile'::text AS subject_type, p.uuid::text AS subject_id, p.id AS profile_id, ''::text AS route_type
    FROM profiles p WHERE p.account_id = sqlc.arg(account_id) AND p.deleted_at IS NULL
    UNION ALL
    SELECT 'route'::text, r.uuid::text, r.profile_id, r.route_type::text
    FROM routes r WHERE r.account_id = sqlc.arg(account_id) AND r.deleted_at IS NULL
    UNION ALL
    SELECT 'card'::text, c.uuid::text, c.profile_id, ''::text
    FROM nfc_cards c WHERE c.account_id = sqlc.arg(account_id) AND c.deleted_at IS NULL
), scoped AS (
    SELECT subj.subject_type, subj.subject_id, subj.route_type
    FROM subj LEFT JOIN sp ON sp.id = subj.profile_id
    WHERE sp.id IS NOT NULL
       OR (sqlc.arg(all_scope)::boolean AND sqlc.arg(actor_role)::text IN ('owner', 'admin'))
)
SELECT d.bucket_date, scoped.subject_type::text AS subject_type, scoped.route_type::text AS route_type,
       d.event_type, SUM(d.total_count)::bigint AS total
FROM scoped
JOIN event_aggregates_daily d ON d.subject_type = scoped.subject_type AND d.subject_id = scoped.subject_id
WHERE d.bucket_date BETWEEN sqlc.arg(from_date)::date AND sqlc.arg(to_date)::date
GROUP BY 1, 2, 3, 4
ORDER BY 1;

-- name: CompanyAnalyticsBreakdown :many
-- Top dimension values summed over the scope; grand_total is the sum over every value.
WITH sp AS (
    SELECT p.id
    FROM profiles p
    LEFT JOIN company_member_profiles l ON l.account_id = p.account_id AND l.profile_id = p.id
    LEFT JOIN company_member_assignments a ON a.account_id = l.account_id AND a.member_id = l.member_id
    LEFT JOIN company_offices off ON off.account_id = p.account_id AND off.id = a.office_id
    LEFT JOIN company_groups g ON g.account_id = p.account_id AND g.id = a.group_id
    WHERE p.account_id = sqlc.arg(account_id) AND p.deleted_at IS NULL
    AND (sqlc.arg(actor_role)::text IN ('owner', 'admin')
        OR (sqlc.arg(actor_role)::text = 'office_manager' AND EXISTS (
            SELECT 1 FROM account_member_scopes s JOIN account_members actor ON actor.id = s.member_id
            WHERE s.account_id = p.account_id AND actor.user_id = sqlc.arg(actor_id)::bigint
              AND s.office_id = a.office_id)))
    AND (sqlc.narg(office_uuid)::uuid IS NULL OR off.uuid = sqlc.narg(office_uuid)::uuid)
    AND (sqlc.narg(group_uuid)::uuid IS NULL OR g.uuid = sqlc.narg(group_uuid)::uuid)
), subj AS (
    SELECT 'profile'::text AS subject_type, p.uuid::text AS subject_id, p.id AS profile_id
    FROM profiles p WHERE p.account_id = sqlc.arg(account_id) AND p.deleted_at IS NULL
    UNION ALL
    SELECT 'route'::text, r.uuid::text, r.profile_id
    FROM routes r WHERE r.account_id = sqlc.arg(account_id) AND r.deleted_at IS NULL
    UNION ALL
    SELECT 'card'::text, c.uuid::text, c.profile_id
    FROM nfc_cards c WHERE c.account_id = sqlc.arg(account_id) AND c.deleted_at IS NULL
), scoped AS (
    SELECT subj.subject_type, subj.subject_id
    FROM subj LEFT JOIN sp ON sp.id = subj.profile_id
    WHERE sp.id IS NOT NULL
       OR (sqlc.arg(all_scope)::boolean AND sqlc.arg(actor_role)::text IN ('owner', 'admin'))
)
SELECT kv.key::text AS key, SUM(kv.value::bigint)::bigint AS count,
       (SUM(SUM(kv.value::bigint)) OVER ())::bigint AS grand_total
FROM scoped
JOIN event_aggregates_daily d ON d.subject_type = scoped.subject_type AND d.subject_id = scoped.subject_id
CROSS JOIN LATERAL jsonb_each_text(CASE sqlc.arg(dimension)::text
    WHEN 'country' THEN d.country_breakdown
    WHEN 'city' THEN d.city_breakdown
    WHEN 'device' THEN d.device_breakdown
    WHEN 'browser' THEN d.browser_breakdown
    WHEN 'os' THEN d.os_breakdown
    WHEN 'referer' THEN d.referer_breakdown
    ELSE '{}'::jsonb END) kv
WHERE d.bucket_date BETWEEN sqlc.arg(from_date)::date AND sqlc.arg(to_date)::date
  AND (sqlc.narg(event_type)::text IS NULL OR d.event_type = sqlc.narg(event_type)::text)
  AND kv.key <> ''
GROUP BY kv.key
ORDER BY count DESC, key
LIMIT sqlc.arg(limit_count);

-- name: CompanyAnalyticsLeads :many
-- Leads per attributed profile and UTC day. A lead points at a profile, a card or a route;
-- the first one set decides the profile (lead, then card, then route).
WITH sp AS (
    SELECT p.id
    FROM profiles p
    LEFT JOIN company_member_profiles l ON l.account_id = p.account_id AND l.profile_id = p.id
    LEFT JOIN company_member_assignments a ON a.account_id = l.account_id AND a.member_id = l.member_id
    LEFT JOIN company_offices off ON off.account_id = p.account_id AND off.id = a.office_id
    LEFT JOIN company_groups g ON g.account_id = p.account_id AND g.id = a.group_id
    WHERE p.account_id = sqlc.arg(account_id) AND p.deleted_at IS NULL
    AND (sqlc.arg(actor_role)::text IN ('owner', 'admin')
        OR (sqlc.arg(actor_role)::text = 'office_manager' AND EXISTS (
            SELECT 1 FROM account_member_scopes s JOIN account_members actor ON actor.id = s.member_id
            WHERE s.account_id = p.account_id AND actor.user_id = sqlc.arg(actor_id)::bigint
              AND s.office_id = a.office_id)))
    AND (sqlc.narg(office_uuid)::uuid IS NULL OR off.uuid = sqlc.narg(office_uuid)::uuid)
    AND (sqlc.narg(group_uuid)::uuid IS NULL OR g.uuid = sqlc.narg(group_uuid)::uuid)
), attributed AS (
    SELECT ld.created_at, sp.id AS profile_id
    FROM leads ld
    LEFT JOIN nfc_cards c ON c.account_id = ld.account_id AND c.id = ld.card_id
    LEFT JOIN routes r ON r.account_id = ld.account_id AND r.id = ld.route_id
    LEFT JOIN sp ON sp.id = COALESCE(ld.profile_id, c.profile_id, r.profile_id)
    WHERE ld.account_id = sqlc.arg(account_id)
      AND ld.created_at >= sqlc.arg(from_date)::date
      AND ld.created_at < (sqlc.arg(to_date)::date + 1)
)
SELECT COALESCE(attributed.profile_id, 0)::bigint AS profile_id,
       (attributed.created_at AT TIME ZONE 'UTC')::date AS day, COUNT(*)::bigint AS total
FROM attributed
WHERE attributed.profile_id IS NOT NULL
   OR (sqlc.arg(all_scope)::boolean AND sqlc.arg(actor_role)::text IN ('owner', 'admin'))
GROUP BY 1, 2;

-- name: CompanyAnalyticsProfiles :many
-- The comparison rows: every scoped live profile with its linked employee, office and group.
SELECT p.id, p.uuid,
       COALESCE(NULLIF(btrim(COALESCE(t.first_name, '') || ' ' || COALESCE(t.last_name, '')), ''),
                NULLIF(t.company_name, ''), '')::text AS display_name,
       COALESCE(t.title, '')::text AS title,
       u.uuid AS member_uuid,
       COALESCE(NULLIF(btrim(COALESCE(u.name, '') || ' ' || COALESCE(u.surname, '')), ''), '')::text AS member_name,
       COALESCE(u.email, '')::text AS member_email,
       off.uuid AS office_uuid, COALESCE(off.name, '')::text AS office_name,
       g.uuid AS group_uuid, COALESCE(g.name, '')::text AS group_name
FROM profiles p
LEFT JOIN LATERAL (
    SELECT pt.first_name, pt.last_name, pt.company_name, pt.title FROM profile_translations pt
    WHERE pt.profile_id = p.id
    ORDER BY pt.is_default DESC, (pt.locale = p.primary_locale) DESC, pt.id LIMIT 1
) t ON true
LEFT JOIN company_member_profiles l ON l.account_id = p.account_id AND l.profile_id = p.id
LEFT JOIN account_members m ON m.account_id = l.account_id AND m.id = l.member_id
LEFT JOIN users u ON u.id = m.user_id
LEFT JOIN company_member_assignments a ON a.account_id = l.account_id AND a.member_id = l.member_id
LEFT JOIN company_offices off ON off.account_id = p.account_id AND off.id = a.office_id
LEFT JOIN company_groups g ON g.account_id = p.account_id AND g.id = a.group_id
WHERE p.account_id = sqlc.arg(account_id) AND p.deleted_at IS NULL
AND (sqlc.arg(actor_role)::text IN ('owner', 'admin')
    OR (sqlc.arg(actor_role)::text = 'office_manager' AND EXISTS (
        SELECT 1 FROM account_member_scopes s JOIN account_members actor ON actor.id = s.member_id
        WHERE s.account_id = p.account_id AND actor.user_id = sqlc.arg(actor_id)::bigint
          AND s.office_id = a.office_id)))
AND (sqlc.narg(office_uuid)::uuid IS NULL OR off.uuid = sqlc.narg(office_uuid)::uuid)
AND (sqlc.narg(group_uuid)::uuid IS NULL OR g.uuid = sqlc.narg(group_uuid)::uuid)
AND (sqlc.arg(q)::text = ''
    OR t.first_name ILIKE '%' || sqlc.arg(q)::text || '%'
    OR t.last_name ILIKE '%' || sqlc.arg(q)::text || '%'
    OR (u.name || ' ' || u.surname) ILIKE '%' || sqlc.arg(q)::text || '%'
    OR u.email ILIKE '%' || sqlc.arg(q)::text || '%')
ORDER BY p.id;
