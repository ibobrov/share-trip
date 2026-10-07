\getenv exporter_password POSTGRES_EXPORTER_PASSWORD

BEGIN;

-- Создаем пользователя, только если он еще не существует.
SELECT 'CREATE ROLE postgres_exporter LOGIN'
WHERE NOT EXISTS (
    SELECT 1
    FROM pg_roles
    WHERE rolname = 'postgres_exporter'
)
\gexec

-- Устанавливаем пароль из переменной окружения.
SELECT format(
               'ALTER ROLE postgres_exporter WITH LOGIN PASSWORD %L',
               :'exporter_password'
       )
\gexec

-- Выдаем доступ к статистике PostgreSQL.
GRANT pg_monitor TO postgres_exporter;

-- Разрешаем подключение к базе, выбранной через POSTGRES_DB.
SELECT format(
               'GRANT CONNECT ON DATABASE %I TO postgres_exporter',
               current_database()
       )
\gexec

COMMIT;