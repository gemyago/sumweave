DO
$$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'sumweave_migrator') THEN
    CREATE ROLE sumweave_migrator LOGIN PASSWORD 'sumweave_migrator_local';
  ELSE
    ALTER ROLE sumweave_migrator WITH LOGIN PASSWORD 'sumweave_migrator_local';
  END IF;

  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'sumweave_runtime') THEN
    CREATE ROLE sumweave_runtime LOGIN PASSWORD 'sumweave_runtime_local';
  ELSE
    ALTER ROLE sumweave_runtime WITH LOGIN PASSWORD 'sumweave_runtime_local';
  END IF;
END
$$;

GRANT CONNECT ON DATABASE sumweave_local TO sumweave_migrator;
GRANT CONNECT ON DATABASE sumweave_local TO sumweave_runtime;

GRANT USAGE, CREATE ON SCHEMA public TO sumweave_migrator;
GRANT USAGE ON SCHEMA public TO sumweave_runtime;

ALTER DEFAULT PRIVILEGES FOR ROLE sumweave_migrator IN SCHEMA public
  GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO sumweave_runtime;

ALTER DEFAULT PRIVILEGES FOR ROLE sumweave_migrator IN SCHEMA public
  GRANT USAGE, SELECT, UPDATE ON SEQUENCES TO sumweave_runtime;
