-- Creates the CRM database and the crm_backoffice role (safe to re-run).
-- Run as a Postgres superuser, e.g.:
--   psql -h localhost -U postgres -d postgres -f scripts/setup-db.sql
-- The password is asked interactively (\password) and never stored in this file.
-- Schema "crm" and all tables are created by backoffice-api migrations on first start.

\set ON_ERROR_STOP on

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'crm_backoffice') THEN
    CREATE ROLE crm_backoffice LOGIN;
  END IF;
END
$$;

\echo 'Set the password for crm_backoffice (put the same value in backoffice-api/.env as CRM_DB_PASSWORD):'
\password crm_backoffice

SELECT 'CREATE DATABASE crm OWNER crm_backoffice ENCODING ''UTF8'''
WHERE NOT EXISTS (SELECT 1 FROM pg_database WHERE datname = 'crm')
\gexec

\connect crm
REVOKE ALL ON DATABASE crm FROM PUBLIC;
GRANT CONNECT, CREATE, TEMPORARY ON DATABASE crm TO crm_backoffice;

\echo 'Done: database crm owned by crm_backoffice.'
