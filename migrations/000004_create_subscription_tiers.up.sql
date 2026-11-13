-- Subscription tiers created by an author (referrer).
-- Stripe columns (stripe_product_id, stripe_price_id, is_published) come in a later migration.
CREATE TABLE subscription_tiers(
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    author_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    level SMALLINT NOT NULL CHECK (level >= 1),
    name VARCHAR(100) NOT NULL,
    description TEXT,
    price_amount INTEGER NOT NULL CHECK (price_amount > 0), -- minor units (cents)
    currency CHAR(3) NOT NULL CHECK (currency = lower(currency)),
    is_archived BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ,
    UNIQUE (author_id, level)
);

-- NULL = free post. Default NO ACTION (not RESTRICT) on purpose: when a user is deleted,
-- both their posts and tiers are removed by cascades in one statement; NO ACTION is checked
-- at the end of the statement, RESTRICT immediately (and would fail depending on cascade order).
ALTER TABLE posts
    ADD COLUMN min_tier_id UUID REFERENCES subscription_tiers(id);

CREATE INDEX idx_posts_min_tier ON posts(min_tier_id) WHERE min_tier_id IS NOT NULL;

-- All subscriptions: the live one + history. Filled manually until Stripe is connected.
-- Stripe columns (stripe_subscription_id, cancel_at_period_end, ...) come in a later migration.
-- TODO(stripe): users ON DELETE CASCADE is not acceptable once real payments exist (see docs/subscriptions-db-design.md).
CREATE TABLE subscriptions(
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    subscriber_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    author_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    tier_id UUID NOT NULL REFERENCES subscription_tiers(id),
    status TEXT NOT NULL CHECK (status IN (
        'incomplete', 'incomplete_expired', 'trialing', 'active',
        'past_due', 'unpaid', 'canceled', 'paused'
    )),
    current_period_end TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (subscriber_id <> author_id)
);

-- at most one live subscription per user; other rows are history
CREATE UNIQUE INDEX uniq_subscriptions_live_subscriber
    ON subscriptions(subscriber_id)
    WHERE status IN ('incomplete', 'trialing', 'active', 'past_due');

CREATE INDEX idx_subscriptions_author_status ON subscriptions(author_id, status);
