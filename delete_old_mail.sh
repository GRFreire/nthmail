#!/bin/sh

if [ -z "$DB_PATH" ]; then
    DB_PATH="db.db"
fi

min_date="$(date -d '-1 day' +%s)"
sql="DELETE FROM mails WHERE mails.arrived_at < $min_date;"

echo "$sql" | sqlite3 "$DB_PATH"

