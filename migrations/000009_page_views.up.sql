-- Profile views: unique visitors per page per day (UTC).
-- page_view_visitors only dedupes "this visitor already counted today": it keeps a salted hash
-- (logged-in: user id, anonymous: ip + user agent), never the ip itself, and rows older than a day
-- are deleted by a periodic job. page_view_daily keeps the numbers (enough for a chart later).

CREATE TABLE page_view_visitors(
    page_user_id UUID NOT NULL REFERENCES profile_pages(user_id) ON DELETE CASCADE,
    day DATE NOT NULL,
    visitor_hash BYTEA NOT NULL,
    PRIMARY KEY (page_user_id, day, visitor_hash)
);

CREATE INDEX idx_page_view_visitors_day ON page_view_visitors(day);

CREATE TABLE page_view_daily(
    page_user_id UUID NOT NULL REFERENCES profile_pages(user_id) ON DELETE CASCADE,
    day DATE NOT NULL,
    views INTEGER NOT NULL DEFAULT 0 CHECK (views >= 0),
    PRIMARY KEY (page_user_id, day)
);
