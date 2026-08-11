UPDATE acs.routers SET
    first_seen_at = COALESCE(first_seen_at, created_at),
    last_seen_at = COALESCE(last_seen_at, created_at),
    last_inform_at = COALESCE(last_inform_at, created_at);
ALTER TABLE acs.routers
    ALTER COLUMN first_seen_at SET DEFAULT now(),
    ALTER COLUMN first_seen_at SET NOT NULL,
    ALTER COLUMN last_seen_at SET DEFAULT now(),
    ALTER COLUMN last_seen_at SET NOT NULL,
    ALTER COLUMN last_inform_at SET DEFAULT now(),
    ALTER COLUMN last_inform_at SET NOT NULL;
