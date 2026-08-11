ALTER TABLE acs.routers
    ALTER COLUMN first_seen_at DROP NOT NULL,
    ALTER COLUMN first_seen_at DROP DEFAULT,
    ALTER COLUMN last_seen_at DROP NOT NULL,
    ALTER COLUMN last_seen_at DROP DEFAULT,
    ALTER COLUMN last_inform_at DROP NOT NULL,
    ALTER COLUMN last_inform_at DROP DEFAULT;
