#!/bin/sh

min_date="$(date -d '-1 day' +%s)"
sql="DELETE FROM mails WHERE mails.arrived_at < $min_date;"

echo "$sql" | sqlite3 db.db

