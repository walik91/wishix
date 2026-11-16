#!/bin/bash
set -euo pipefail

# Quote SQL identifiers from the environment.
quote_identifier() { printf '"%s"' "${1//\"/\"\"}"; }

database=$(quote_identifier "$DB_NAME")
db_user=$(quote_identifier "$DB_USER")
/cockroach/cockroach sql --insecure --host=db:26257 <<SQL
CREATE DATABASE IF NOT EXISTS $database;
CREATE USER IF NOT EXISTS $db_user;
GRANT ALL ON DATABASE $database TO $db_user;
GRANT ALL ON SCHEMA $database.public TO $db_user;
SQL
