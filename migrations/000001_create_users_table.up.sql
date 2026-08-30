CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE users(
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    username VARCHAR(25) NOT NULL, 
    email VARCHAR(255) NOT NULL UNIQUE,
    hashed_password VARCHAR(255) NOT NULL,
    own_referral_key VARCHAR(255) NOT NULL UNIQUE,
    referred_by_id UUID REFERENCES users(id) ON DELETE SET NULL,
    last_time_ref_used TIMESTAMPTZ, 
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE referrals(
    referral_owner UUID REFERENCES users(id),
    referred_user_id UUID REFERENCES users(id),
    referral_timestamp TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (referral_owner, referred_user_id)
);

CREATE TABLE giveaways(
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    title VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE giveaway_attendee(
    gw_id UUID REFERENCES giveaways(id) ON DELETE CASCADE,
    gw_owner_id UUID REFERENCES users(id) ON DELETE CASCADE,
    gw_user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    gw_won BOOLEAN NOT NULL DEFAULT FALSE,
    joined_at TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (gw_id, gw_user_id)
);

