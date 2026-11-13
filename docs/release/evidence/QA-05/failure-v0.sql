CREATE TRIGGER fail_legacy_schema_migration
BEFORE UPDATE ON records
BEGIN
    SELECT RAISE(ABORT, 'injected legacy schema migration failure');
END;
