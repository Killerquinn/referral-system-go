-- Public customizable author page (guns.lol-like). One page per user, created explicitly.
-- slug = "second username": the page address, independent from users.username.
-- Lowercase-only regex keeps a plain UNIQUE enough (no lower(slug) index needed).
CREATE TABLE profile_pages(
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    slug VARCHAR(32) NOT NULL UNIQUE CHECK (slug ~ '^[a-z0-9_-]{3,32}$'),
    display_name VARCHAR(64),
    bio VARCHAR(500),
    settings JSONB NOT NULL DEFAULT '{}'::jsonb,
    settings_version SMALLINT NOT NULL DEFAULT 1,
    is_published BOOLEAN NOT NULL DEFAULT FALSE,
    slug_changed_at TIMESTAMPTZ, -- for the slug change cooldown, NULL = never changed
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ
);

-- referral counter on the page counts current referrals (users.referred_by_id)
CREATE INDEX IF NOT EXISTS idx_users_referred_by_id ON users(referred_by_id);
