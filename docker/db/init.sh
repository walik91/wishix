#!/bin/bash
set -euo pipefail

# Quote SQL identifiers and string literals without placing the password in argv.
quote_identifier() { printf '"%s"' "${1//\"/\"\"}"; }
quote_literal() { printf "'%s'" "${1//\'/\'\'}"; }

database=$(quote_identifier "$DB_NAME")
db_user=$(quote_identifier "$DB_USER")
password=$(quote_literal "$DB_PASSWORD")

# The application receives only the public CA, never root or node private keys.
cp /cockroach/certs/ca.crt /db-certs/ca.crt
chmod 644 /db-certs/ca.crt

/cockroach/cockroach sql --certs-dir=/cockroach/certs --host=db:26257 <<SQL
CREATE DATABASE IF NOT EXISTS $database;
CREATE USER IF NOT EXISTS $db_user;
ALTER USER $db_user WITH PASSWORD $password;
GRANT ALL ON DATABASE $database TO $db_user;
GRANT ALL ON SCHEMA $database.public TO $db_user;
SQL
