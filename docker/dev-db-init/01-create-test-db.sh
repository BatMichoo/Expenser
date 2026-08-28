#!/bin/sh
set -e

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-EOSQL
    SELECT 'CREATE DATABASE "${TEST_DB_NAME}"'
    WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = '${TEST_DB_NAME}')\gexec
EOSQL
