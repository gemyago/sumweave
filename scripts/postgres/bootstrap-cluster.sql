DO
$$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'sumweave_owner') THEN
    CREATE ROLE sumweave_owner LOGIN PASSWORD 'sumweave_owner_local';
  ELSE
    ALTER ROLE sumweave_owner WITH LOGIN PASSWORD 'sumweave_owner_local';
  END IF;

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

SELECT format('CREATE DATABASE %I OWNER %I', database_name, 'sumweave_owner')
FROM (VALUES ('sumweave_local'), ('sumweave_test')) AS databases(database_name)
WHERE NOT EXISTS (SELECT 1 FROM pg_database WHERE datname = database_name)
\gexec

ALTER DATABASE sumweave_local OWNER TO sumweave_owner;
ALTER DATABASE sumweave_test OWNER TO sumweave_owner;
