
CREATE TABLE post_embeds(
    post_id UUID NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    position SMALLINT NOT NULL CHECK(position >= 0),
    provider TEXT NOT NULL CHECK (provider IN ('youtube', 'vimeo', 'tiktok', 'twitch_video', 'twitch_clip')),
    external_id TEXT NOT NULL CHECK (length(external_id) BETWEEN 1 AND 100),

    PRIMARY KEY(post_id, position),
    UNIQUE(post_id, provider, external_id)
);