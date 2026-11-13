DROP TABLE IF EXISTS subscriptions;
ALTER TABLE posts DROP COLUMN IF EXISTS min_tier_id;
DROP TABLE IF EXISTS subscription_tiers;
