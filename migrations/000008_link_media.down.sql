ALTER TABLE profile_pages
    DROP COLUMN IF EXISTS music_upload_id,
    DROP COLUMN IF EXISTS background_upload_id;

ALTER TABLE users
    DROP COLUMN IF EXISTS avatar_upload_id;
