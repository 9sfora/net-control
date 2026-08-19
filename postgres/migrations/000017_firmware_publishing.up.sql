ALTER TABLE acs.firmwares ALTER COLUMN active SET DEFAULT FALSE;

UPDATE acs.firmwares
SET active = FALSE,
    is_default = FALSE
WHERE status = 'NO_FILE';
