-- refresh/logout look sessions up by token hash
CREATE UNIQUE INDEX IF NOT EXISTS idx_user_sessions_refresh_token_hash ON user_sessions(refresh_token_hash);

-- logout-all, password change and ON DELETE CASCADE from users delete sessions by user_id
CREATE INDEX IF NOT EXISTS idx_user_sessions_user_id ON user_sessions(user_id);
