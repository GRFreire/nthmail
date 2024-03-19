#!/bin/sh

pid=0

get_ts() {
    stat cmd/web_server pkg/** | grep Modify | awk '{$1=""; print $0}' | sed 's/^ //g' | sort -r | head -1
}

run_server() {
    make -B web
    ./bin/web_server &
    pid=$!
}

ts="$(get_ts)"
run_server

trap "kill -s KILL $pid; trap - EXIT; exit" EXIT INT HUP

while true; do
    sleep 1;
    new_ts="$(get_ts)"
    if [ "$ts" != "$new_ts" ]; then
        ts="$new_ts"
        echo ""
        if [ "$pid" != "0" ]; then
            kill -s KILL $pid
        fi
        run_server
    fi
done

