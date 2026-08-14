-- One-time PostgreSQL upgrade: legacy appdispatch -> topic-aware appdispatch.
--
-- Run this with psql, from a DDL-capable role, immediately before deploying
-- the PR #5 application version. This script is intentionally not
-- idempotent: it refuses a database that does not have exactly the legacy
-- dispatch shape, so it cannot silently overwrite a completed or different
-- migration.
--
-- Preconditions
--   1. Take a verified, restorable PostgreSQL backup first.
--   2. Stop every old application API, jobs worker, and scheduler. Do not run
--      old and new binaries against the database at the same time.
--   3. Run this before the Helm pre-upgrade db-migrate Job. db-migrate creates
--      the target schema on an empty database, but does not transform existing
--      legacy tables.
--   4. The legacy worker group must be sumweave-app-dispatch. That is the only
--      group created by the pre-PR application. A different group means this
--      script cannot safely decide its replacement and will abort.
--   5. Identify the IANA time zone used by the legacy application's database
--      connections, edit the marked literal below, and have an operator review
--      that edit. The checked-in placeholder deliberately aborts.
--
-- The script is for the configured application database, not the separate
-- agent-runtime database. By default it addresses the production table prefix
-- from default.yaml. Run it from the repository root:
--
--   psql "$APP_APPLICATION_DATABASE_DSN" -f docs/appdispatch-postgres-topic-upgrade.sql
--
-- This artifact deliberately targets the production default in public:
-- sumweave_app_dispatch_messages and sumweave_app_dispatch_offsets. A custom
-- tablePrefix or PostgreSQL schema is an explicit precondition failure; make a
-- reviewed copy with every table and expected sequence identifier changed
-- before running it.
--
-- Data and delivery semantics
--   * All legacy rows were jobs commands on app.dispatch.execution.v1. They
--     become jobs.execution.v1, the PR #5 jobs topic.
--   * The old sole consumer position is renamed from sumweave-app-dispatch to
--     jobs.workers.v1. Its offset and transaction ID are retained, so consumed
--     commands stay consumed and pending commands remain pending.
--   * This is at-least-once transport. A crash after a handler side effect but
--     before acknowledgement may still redeliver work; the jobs store handles
--     non-queued redelivery as a no-op.
--   * Legacy created_at is a timestamp without time zone: it is a wall-clock
--     value, not an instant. Before running, identify and explicitly confirm
--     the IANA time zone that application connections used when those values
--     were written (for example, Europe/Warsaw). Change the one placeholder in
--     the SET LOCAL statement below. The script rejects the placeholder,
--     invalid values, and non-IANA-style zone names before any DDL.
--   * The conversion uses that confirmed value only; it never reads the psql
--     session's TimeZone setting. A timestamp in a daylight-saving fall-back
--     overlap is inherently ambiguous after it was stored without a zone.
--     PostgreSQL resolves such a value using its standard-time offset. Confirm
--     that this rule matches the legacy writer, or correct those rows from
--     independent evidence before running this script.
--
-- The transaction takes ACCESS EXCLUSIVE locks. Expected downtime covers the
-- whole script plus deployment. If a stopped workload still holds a connection,
-- the five-second lock timeout aborts without changing anything. On any error,
-- psql's ON_ERROR_STOP and the transaction rollback leave the legacy schema
-- intact. Restore the backup only if an operator has separately changed data
-- outside this transaction; ordinary script failures need no rollback action.

\set ON_ERROR_STOP on
BEGIN;
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '15min';
-- OPERATOR ACTION REQUIRED: replace this placeholder with the confirmed
-- historical IANA zone used by legacy application connections, then review
-- this file as an operational change. Example: 'Europe/Warsaw'.
SET LOCAL appdispatch.legacy_created_at_timezone = 'REPLACE_WITH_HISTORICAL_IANA_TIME_ZONE';

DO $timezone_validation$
DECLARE
    historical_timezone text := current_setting('appdispatch.legacy_created_at_timezone');
BEGIN
    IF historical_timezone = 'REPLACE_WITH_HISTORICAL_IANA_TIME_ZONE' THEN
        RAISE EXCEPTION
            'set appdispatch.legacy_created_at_timezone to the confirmed historical IANA time zone before running this script';
    END IF;

    -- Require a geographic/Etc IANA identifier (or canonical UTC), not a
    -- session-dependent abbreviation such as CET or EST.
    IF (historical_timezone <> 'UTC' AND historical_timezone !~ '^[A-Za-z0-9._+-]+(/[A-Za-z0-9._+-]+)+$')
       OR NOT EXISTS (SELECT 1 FROM pg_timezone_names WHERE name = historical_timezone) THEN
        RAISE EXCEPTION
            'appdispatch.legacy_created_at_timezone must be a valid IANA time zone (for example Europe/Warsaw or Etc/UTC), got %',
            historical_timezone;
    END IF;
END
$timezone_validation$;

LOCK TABLE public.sumweave_app_dispatch_messages,
            public.sumweave_app_dispatch_offsets IN ACCESS EXCLUSIVE MODE;

DO $$
DECLARE
    messages_oid regclass := 'public.sumweave_app_dispatch_messages'::regclass;
    offsets_oid regclass := 'public.sumweave_app_dispatch_offsets'::regclass;
    messages_offset_attnum smallint;
    messages_transaction_id_attnum smallint;
    offsets_consumer_group_attnum smallint;
    offsets_primary_key text;
    unexpected_groups text;
BEGIN
    IF EXISTS (
        SELECT 1 FROM pg_attribute
        WHERE attrelid = messages_oid AND attname = 'topic' AND attnum > 0 AND NOT attisdropped
    ) OR EXISTS (
        SELECT 1 FROM pg_attribute
        WHERE attrelid = offsets_oid AND attname = 'topic' AND attnum > 0 AND NOT attisdropped
    ) THEN
        RAISE EXCEPTION 'topic column already exists; this is not the legacy schema and this one-time script must not be rerun';
    END IF;

    IF (SELECT count(*) FROM pg_attribute
        WHERE attrelid = messages_oid AND attnum > 0 AND NOT attisdropped) <> 6
    OR (SELECT count(*) FROM pg_attribute WHERE attrelid = messages_oid AND NOT attisdropped
        AND attnum > 0 AND attname = ANY (ARRAY['offset', 'uuid', 'created_at', 'payload', 'metadata', 'transaction_id'])) <> 6
    OR (SELECT count(*) FROM pg_attribute
        WHERE attrelid = offsets_oid AND attnum > 0 AND NOT attisdropped) <> 3
    OR (SELECT count(*) FROM pg_attribute WHERE attrelid = offsets_oid
        AND attnum > 0 AND NOT attisdropped
        AND attname = ANY (ARRAY['consumer_group', 'offset_acked', 'last_processed_transaction_id'])) <> 3 THEN
        RAISE EXCEPTION 'tables do not contain the expected pre-PR appdispatch columns';
    END IF;

    IF EXISTS (
        WITH expected(attname, type_name, is_not_null, default_expression) AS (
            VALUES
                ('uuid', 'character varying(36)', true, NULL::text),
                ('created_at', 'timestamp without time zone', true, 'CURRENT_TIMESTAMP'),
                ('payload', 'bytea', false, NULL::text),
                ('metadata', 'json', false, NULL::text),
                ('transaction_id', 'xid8', true, NULL::text)
        ), actual AS (
            SELECT a.attname,
                   format_type(a.atttypid, a.atttypmod) AS type_name,
                   a.attnotnull AS is_not_null,
                   pg_get_expr(d.adbin, d.adrelid) AS default_expression,
                   a.attidentity,
                   a.attgenerated
            FROM pg_attribute a
            LEFT JOIN pg_attrdef d ON d.adrelid = a.attrelid AND d.adnum = a.attnum
            WHERE a.attrelid = messages_oid AND a.attnum > 0 AND NOT a.attisdropped
        )
        SELECT 1
        FROM expected e
        LEFT JOIN actual a USING (attname)
        WHERE a.attname IS NULL
           OR a.type_name IS DISTINCT FROM e.type_name
           OR a.is_not_null IS DISTINCT FROM e.is_not_null
           OR a.default_expression IS DISTINCT FROM e.default_expression
           OR a.attidentity <> ''
           OR a.attgenerated <> ''
    ) THEN
        RAISE EXCEPTION
            'legacy messages columns must have the exact expected types, nullability, and defaults';
    END IF;

    IF EXISTS (
        WITH expected(attname, type_name, is_not_null, default_expression) AS (
            VALUES
                ('consumer_group', 'character varying(255)', true, NULL::text),
                ('offset_acked', 'bigint', false, NULL::text),
                ('last_processed_transaction_id', 'xid8', true, NULL::text)
        ), actual AS (
            SELECT a.attname,
                   format_type(a.atttypid, a.atttypmod) AS type_name,
                   a.attnotnull AS is_not_null,
                   pg_get_expr(d.adbin, d.adrelid) AS default_expression,
                   a.attidentity,
                   a.attgenerated
            FROM pg_attribute a
            LEFT JOIN pg_attrdef d ON d.adrelid = a.attrelid AND d.adnum = a.attnum
            WHERE a.attrelid = offsets_oid AND a.attnum > 0 AND NOT a.attisdropped
        )
        SELECT 1
        FROM expected e
        LEFT JOIN actual a USING (attname)
        WHERE a.attname IS NULL
           OR a.type_name IS DISTINCT FROM e.type_name
           OR a.is_not_null IS DISTINCT FROM e.is_not_null
           OR a.default_expression IS DISTINCT FROM e.default_expression
           OR a.attidentity <> ''
           OR a.attgenerated <> ''
    ) THEN
        RAISE EXCEPTION
            'legacy offsets columns must have the exact expected types, nullability, and defaults';
    END IF;

    SELECT attnum INTO messages_offset_attnum
    FROM pg_attribute
    WHERE attrelid = messages_oid AND attname = 'offset' AND attnum > 0 AND NOT attisdropped;
    SELECT attnum INTO messages_transaction_id_attnum
    FROM pg_attribute
    WHERE attrelid = messages_oid AND attname = 'transaction_id' AND attnum > 0 AND NOT attisdropped;
    SELECT attnum INTO offsets_consumer_group_attnum
    FROM pg_attribute
    WHERE attrelid = offsets_oid AND attname = 'consumer_group' AND attnum > 0 AND NOT attisdropped;

    IF EXISTS (
        SELECT 1
        FROM pg_attribute a
        WHERE a.attrelid = messages_oid
          AND a.attnum = messages_offset_attnum
          AND (format_type(a.atttypid, a.atttypmod) <> 'bigint'
               OR NOT a.attnotnull
               OR a.attidentity <> ''
               OR a.attgenerated <> '')
    ) THEN
        RAISE EXCEPTION
            'legacy messages offset must be BIGSERIAL with no identity or generated expression';
    END IF;

    IF NOT EXISTS (
        SELECT 1
        FROM pg_attribute a
        JOIN pg_attrdef d ON d.adrelid = a.attrelid AND d.adnum = a.attnum
        JOIN pg_depend default_dependency
          ON default_dependency.classid = 'pg_attrdef'::regclass
         AND default_dependency.objid = d.oid
         AND default_dependency.refclassid = 'pg_class'::regclass
        JOIN pg_class sequence ON sequence.oid = default_dependency.refobjid
        JOIN pg_namespace sequence_namespace ON sequence_namespace.oid = sequence.relnamespace
        JOIN pg_depend sequence_owner
          ON sequence_owner.classid = 'pg_class'::regclass
         AND sequence_owner.objid = sequence.oid
         AND sequence_owner.refclassid = 'pg_class'::regclass
         AND sequence_owner.refobjid = messages_oid
         AND sequence_owner.refobjsubid = a.attnum
         AND sequence_owner.deptype = 'a'
        WHERE a.attrelid = messages_oid
          AND a.attnum = messages_offset_attnum
          AND sequence.relkind = 'S'
          AND sequence_namespace.nspname = 'public'
          AND sequence.relname = 'sumweave_app_dispatch_messages_offset_seq'
          AND pg_get_expr(d.adbin, d.adrelid) LIKE 'nextval(%::regclass)'
    ) THEN
        RAISE EXCEPTION
            'legacy messages offset must be the expected BIGSERIAL default backed by its owned sequence';
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conrelid = messages_oid
          AND contype = 'p'
          AND cardinality(conkey) = 2
          AND conkey[1] = messages_transaction_id_attnum
          AND conkey[2] = messages_offset_attnum
    ) THEN
        RAISE EXCEPTION 'legacy messages primary key must be (transaction_id, offset)';
    END IF;

    SELECT conname INTO offsets_primary_key
    FROM pg_constraint
    WHERE conrelid = offsets_oid
      AND contype = 'p'
      AND cardinality(conkey) = 1
      AND conkey[1] = offsets_consumer_group_attnum;
    IF offsets_primary_key IS NULL THEN
        RAISE EXCEPTION 'legacy offsets primary key must be (consumer_group)';
    END IF;

    SELECT string_agg(consumer_group, ', ' ORDER BY consumer_group) INTO unexpected_groups
    FROM (SELECT DISTINCT consumer_group FROM public.sumweave_app_dispatch_offsets
          WHERE consumer_group <> 'sumweave-app-dispatch') AS groups;
    IF unexpected_groups IS NOT NULL THEN
        RAISE EXCEPTION 'unsupported legacy consumer group(s): %', unexpected_groups;
    END IF;

    EXECUTE format('ALTER TABLE public.sumweave_app_dispatch_offsets DROP CONSTRAINT %I', offsets_primary_key);
END $$;

ALTER TABLE public.sumweave_app_dispatch_messages ADD COLUMN topic VARCHAR(255);
UPDATE public.sumweave_app_dispatch_messages SET topic = 'jobs.execution.v1';
ALTER TABLE public.sumweave_app_dispatch_messages ALTER COLUMN topic SET NOT NULL;
ALTER TABLE public.sumweave_app_dispatch_messages
    ALTER COLUMN created_at DROP DEFAULT,
    ALTER COLUMN created_at TYPE TIMESTAMP WITH TIME ZONE
        USING created_at AT TIME ZONE current_setting('appdispatch.legacy_created_at_timezone'),
    ALTER COLUMN created_at SET DEFAULT CURRENT_TIMESTAMP;
CREATE INDEX sumweave_app_dispatch_messages_topic_order_idx
    ON public.sumweave_app_dispatch_messages (topic, transaction_id, "offset");

ALTER TABLE public.sumweave_app_dispatch_offsets ADD COLUMN topic VARCHAR(255);
UPDATE public.sumweave_app_dispatch_offsets
    SET topic = 'jobs.execution.v1', consumer_group = 'jobs.workers.v1';
ALTER TABLE public.sumweave_app_dispatch_offsets ALTER COLUMN topic SET NOT NULL;
ALTER TABLE public.sumweave_app_dispatch_offsets
    ADD PRIMARY KEY (topic, consumer_group);

-- Transactional checks before making the upgrade durable.
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM public.sumweave_app_dispatch_messages
        WHERE topic <> 'jobs.execution.v1'
    ) OR EXISTS (
        SELECT 1 FROM public.sumweave_app_dispatch_offsets
        WHERE topic <> 'jobs.execution.v1' OR consumer_group <> 'jobs.workers.v1'
    ) THEN
        RAISE EXCEPTION 'post-upgrade appdispatch topic or consumer group validation failed';
    END IF;
END $$;

COMMIT;

-- After COMMIT, deploy the PR #5 image. Its migration Job can safely run
-- sumweave db-migrate; it sees this target schema and creates other new tables.
-- Do not roll back only the binary afterward: the old binary cannot use this
-- schema. If a binary rollback is required, restore the verified pre-upgrade
-- database backup and deploy the matching old image together.
