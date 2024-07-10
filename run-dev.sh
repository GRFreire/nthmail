#!/bin/sh

pid=0

get_ts() {
    stat pkg/** | grep Modify | awk '{$1=""; print $0}' | sed 's/^ //g' | sort -r | head -1
}

run_server() {
    make -B
    ./bin/server &
    pid=$!
}

ts="$(get_ts)"
run_server

k() {
    kill -s KILL $pid
}

trap "k; trap - EXIT; exit 0" EXIT INT HUP

while true; do
    sleep 1;
    new_ts="$(get_ts)"
    if [ "$ts" != "$new_ts" ]; then
        ts="$new_ts"
        echo ""
        if [ "$pid" != "0" ]; then
            k
        fi
        run_server
    fi
done

