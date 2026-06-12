#!/usr/bin/env bash
set -euo pipefail

if [[ -f .env ]]; then
	set -a
	source .env
	set +a
fi

: "${DB_NAME:=app_db}"
: "${DB_USER:=app_user}"
: "${DB_PASS:=app_password}"
: "${DB_ADMIN_DB:=postgres}"
: "${USE_SUDO_POSTGRES:=0}"

if [[ "$USE_SUDO_POSTGRES" == "1" ]]; then
	PSQL_ADMIN=(sudo -u postgres psql -d "$DB_ADMIN_DB")
else
	PSQL_ADMIN=(psql -d "$DB_ADMIN_DB")
fi

sql_literal() {
	printf "'%s'" "${1//\'/\'\'}"
}

echo "Настройка базы данных '$DB_NAME' и пользователя '$DB_USER'..."

if [[ $("${PSQL_ADMIN[@]}" -tAc "SELECT 1 FROM pg_roles WHERE rolname = $(sql_literal "$DB_USER")" | tr -d '[:space:]') != "1" ]]; then
	"${PSQL_ADMIN[@]}" -v ON_ERROR_STOP=1 -c "CREATE USER \"$DB_USER\" WITH ENCRYPTED PASSWORD $(sql_literal "$DB_PASS");"
	echo "Пользователь '$DB_USER' создан."
else
	"${PSQL_ADMIN[@]}" -v ON_ERROR_STOP=1 -c "ALTER USER \"$DB_USER\" WITH ENCRYPTED PASSWORD $(sql_literal "$DB_PASS");"
	echo "Пользователь '$DB_USER' уже существует, пароль обновлен."
fi

if [[ $("${PSQL_ADMIN[@]}" -tAc "SELECT 1 FROM pg_database WHERE datname = $(sql_literal "$DB_NAME")" | tr -d '[:space:]') != "1" ]]; then
	"${PSQL_ADMIN[@]}" -v ON_ERROR_STOP=1 -c "CREATE DATABASE \"$DB_NAME\" OWNER \"$DB_USER\";"
	echo "База данных '$DB_NAME' создана."
else
	echo "База данных '$DB_NAME' уже существует."
fi

"${PSQL_ADMIN[@]}" -v ON_ERROR_STOP=1 <<SQL
GRANT CONNECT ON DATABASE "$DB_NAME" TO "$DB_USER";
ALTER DATABASE "$DB_NAME" OWNER TO "$DB_USER";
SQL

if [[ "$USE_SUDO_POSTGRES" == "1" ]]; then
	PSQL_TARGET=(sudo -u postgres psql -d "$DB_NAME")
else
	PSQL_TARGET=(psql -d "$DB_NAME")
fi

"${PSQL_TARGET[@]}" -v ON_ERROR_STOP=1 <<SQL
REVOKE ALL ON SCHEMA public FROM PUBLIC;
GRANT USAGE, CREATE ON SCHEMA public TO "$DB_USER";
GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO "$DB_USER";
GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO "$DB_USER";
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON TABLES TO "$DB_USER";
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON SEQUENCES TO "$DB_USER";
SQL

echo "Готово: база '$DB_NAME' доступна пользователю '$DB_USER'."
