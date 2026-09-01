\connect sumweave_local

GRANT CONNECT ON DATABASE sumweave_local TO sumweave_migrator;
GRANT CONNECT ON DATABASE sumweave_local TO sumweave_runtime;
GRANT USAGE, CREATE ON SCHEMA public TO sumweave_migrator;
GRANT USAGE ON SCHEMA public TO sumweave_runtime;
ALTER DEFAULT PRIVILEGES FOR ROLE sumweave_migrator IN SCHEMA public
  GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO sumweave_runtime;
ALTER DEFAULT PRIVILEGES FOR ROLE sumweave_migrator IN SCHEMA public
  GRANT USAGE, SELECT, UPDATE ON SEQUENCES TO sumweave_runtime;

\connect sumweave_test

GRANT CONNECT ON DATABASE sumweave_test TO sumweave_migrator;
GRANT CONNECT ON DATABASE sumweave_test TO sumweave_runtime;
GRANT USAGE, CREATE ON SCHEMA public TO sumweave_migrator;
GRANT USAGE ON SCHEMA public TO sumweave_runtime;
ALTER DEFAULT PRIVILEGES FOR ROLE sumweave_migrator IN SCHEMA public
  GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO sumweave_runtime;
ALTER DEFAULT PRIVILEGES FOR ROLE sumweave_migrator IN SCHEMA public
  GRANT USAGE, SELECT, UPDATE ON SEQUENCES TO sumweave_runtime;
