#!/bin/sh
set -eu

# The application owns its schema for startup migrations, but is not a DB superuser.
psql --username postgres --dbname glipz --set=ON_ERROR_STOP=1 \
  --set=app_password="$GLIPZ_APP_PASSWORD" <<'SQL'
CREATE ROLE glipz LOGIN PASSWORD :'app_password' NOSUPERUSER NOCREATEDB NOCREATEROLE;
ALTER DATABASE glipz OWNER TO glipz;
SQL
psql --username glipz --dbname glipz --set=ON_ERROR_STOP=1 --file=/opt/glipz/init.sql
