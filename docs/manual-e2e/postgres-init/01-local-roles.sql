DO
$$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'signal_foundry_migrator') THEN
    CREATE ROLE signal_foundry_migrator LOGIN PASSWORD 'signal_foundry_migrator_local';
  ELSE
    ALTER ROLE signal_foundry_migrator WITH LOGIN PASSWORD 'signal_foundry_migrator_local';
  END IF;

  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'signal_foundry_runtime') THEN
    CREATE ROLE signal_foundry_runtime LOGIN PASSWORD 'signal_foundry_runtime_local';
  ELSE
    ALTER ROLE signal_foundry_runtime WITH LOGIN PASSWORD 'signal_foundry_runtime_local';
  END IF;
END
$$;

GRANT CONNECT ON DATABASE signal_foundry_local TO signal_foundry_migrator;
GRANT CONNECT ON DATABASE signal_foundry_local TO signal_foundry_runtime;

GRANT USAGE, CREATE ON SCHEMA public TO signal_foundry_migrator;
GRANT USAGE ON SCHEMA public TO signal_foundry_runtime;

ALTER DEFAULT PRIVILEGES FOR ROLE signal_foundry_migrator IN SCHEMA public
  GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO signal_foundry_runtime;

ALTER DEFAULT PRIVILEGES FOR ROLE signal_foundry_migrator IN SCHEMA public
  GRANT USAGE, SELECT, UPDATE ON SEQUENCES TO signal_foundry_runtime;
