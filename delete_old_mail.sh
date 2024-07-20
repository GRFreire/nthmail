#!/bin/sh

if [ -z "$DB_PATH" ]; then
    DB_PATH="db.db"
fi

min_date="$(date -d '-1 day' +%s)"
sql="DELETE FROM mails WHERE mails.arrived_at < $min_date"

if [ -n "$EXCLUDE_IGNORE_ADDR" ]; then
    sql="$sql AND not (rcpt_addr = '$EXCLUDE_IGNORE_ADDR')"
fi
sql="$sql;"

echo "$sql" | sqlite3 "$DB_PATH"

