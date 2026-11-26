-- TEC-1315: a member shares one of their personal profiles with a company they belong to.
-- Shared profiles are listed read-only to the company's owners and admins. Sharing is per
-- profile and per company, off by default. The share goes away with the membership (the member
-- leaves or is removed) and with the profile.
CREATE TABLE profile_company_shares (
    id                 BIGSERIAL   PRIMARY KEY,
    -- account_id owns the shared profile (the member's personal account).
    account_id         BIGINT      NOT NULL,
    profile_id         BIGINT      NOT NULL,
    company_account_id BIGINT      NOT NULL,
    member_id          BIGINT      NOT NULL,
    shared_by_user_id  BIGINT      NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_profile_company_shares UNIQUE (profile_id, company_account_id),
    CONSTRAINT chk_profile_company_shares_other_account CHECK (account_id <> company_account_id),
    FOREIGN KEY (account_id, profile_id) REFERENCES profiles (account_id, id) ON DELETE CASCADE,
    FOREIGN KEY (company_account_id, member_id) REFERENCES account_members (account_id, id) ON DELETE CASCADE
);

CREATE INDEX idx_profile_company_shares_company ON profile_company_shares (company_account_id, created_at DESC);
CREATE INDEX idx_profile_company_shares_account ON profile_company_shares (account_id);
