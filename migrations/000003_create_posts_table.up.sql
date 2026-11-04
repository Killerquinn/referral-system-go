CREATE TABLE posts(
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    author_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title VARCHAR(200) NOT NULL,
    body TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ
);

-- cursor pagination: WHERE author_id = $1 AND (created_at, id) < ($2, $3) ORDER BY created_at DESC, id DESC
CREATE INDEX idx_posts_author_created ON posts(author_id, created_at DESC, id DESC);
