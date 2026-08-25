-- Creates the second database used by internal/controlplane/store's
-- (and internal/agent's, internal/cpclient's) tests, alongside the
-- "ambud" database POSTGRES_DB already creates for running
-- ambud-controlplane itself. Runs once, only on first container
-- creation — see postgres.yaml's docker-entrypoint-initdb.d mount.
CREATE DATABASE ambud_test OWNER ambud;
