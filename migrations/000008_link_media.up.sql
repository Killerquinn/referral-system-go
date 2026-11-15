-- Links uploaded files to the places that show them. The tables point at uploads(id), never at storage keys:
-- uploads knows where the bytes live (bucket, variants), so moving files later touches only uploads.
-- ON DELETE SET NULL: deleting the file just removes it from the avatar / page.

ALTER TABLE users
    ADD COLUMN avatar_upload_id UUID REFERENCES uploads(id) ON DELETE SET NULL;

ALTER TABLE profile_pages
    ADD COLUMN background_upload_id UUID REFERENCES uploads(id) ON DELETE SET NULL,
    ADD COLUMN music_upload_id UUID REFERENCES uploads(id) ON DELETE SET NULL;

-- deleting an upload makes Postgres look up rows referencing it; without these that's a full scan
CREATE INDEX idx_users_avatar_upload ON users(avatar_upload_id) WHERE avatar_upload_id IS NOT NULL;
CREATE INDEX idx_pages_background_upload ON profile_pages(background_upload_id) WHERE background_upload_id IS NOT NULL;
CREATE INDEX idx_pages_music_upload ON profile_pages(music_upload_id) WHERE music_upload_id IS NOT NULL;
