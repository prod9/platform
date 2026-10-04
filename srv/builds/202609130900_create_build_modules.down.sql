LOCK TABLE builds, build_modules, build_events IN ACCESS EXCLUSIVE MODE;

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM builds)
        OR EXISTS (SELECT 1 FROM build_modules)
        OR EXISTS (SELECT 1 FROM build_events) THEN
        RAISE EXCEPTION 'cannot roll back module transition: new build records exist';
    END IF;
END $$;

-- Even an empty replacement table may have allocated IDs. Keep those across reapplication.
SELECT setval('build_history.builds_id_seq',
    GREATEST(history.last_value, current_ids.last_value),
    history.is_called OR current_ids.is_called)
FROM build_history.builds_id_seq history, builds_id_seq current_ids;

SELECT setval('build_history.build_events_id_seq',
    GREATEST(history.last_value, current_ids.last_value),
    history.is_called OR current_ids.is_called)
FROM build_history.build_events_id_seq history, build_events_id_seq current_ids;

DROP TABLE build_events;
DROP TABLE build_modules;
DROP TABLE builds;

ALTER TABLE build_history.builds SET SCHEMA public;
ALTER TABLE build_history.build_events SET SCHEMA public;
DROP SCHEMA build_history;
