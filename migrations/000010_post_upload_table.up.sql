CREATE TABLE post_uploads(
    post_id UUID NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    upload_id UUID NOT NULL REFERENCES uploads(id) ON DELETE RESTRICT,
    position SMALLINT NOT NULL CHECK(position >= 0),
    caption VARCHAR(255),
    PRIMARY KEY (post_id, upload_id),
    UNIQUE(post_id, position)
);

CREATE INDEX idx_post_uploads_upload_id ON post_uploads(upload_id);