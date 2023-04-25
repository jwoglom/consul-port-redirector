#!/bin/bash

PORT=8686
ERRORS=0

redir_url() {
    curl -Ls -o /dev/null -w %{url_effective} "$@"
}

check() {
    actual=$(redir_url -H "Host: $1" "http://localhost:$PORT/$2")
    expected=$3
    if [[ "$actual" == "$expected" ]]; then
        echo "OK: $actual == $expected"
    else
        echo "FAIL: $actual != $expected"
        ((ERRORS++))
    fi
    echo ""
}

check_no_redir() {
    path=$2
    if [[ "$path" == "" ]]; then path=/; fi
    check "$1" "$2" "http://localhost:$PORT$path"
}

go build main.go
./main \
    --port=$PORT \
    --customRoutes='{
        "h":"http://home:1234",
        "h/grafana":"http://grafana:2345/graphs/index",
        "h/grafana/overview":"http://grafana:2345/graphs/12345",
        "h/graph":"http://grafana:2345/graph?id=$arg$&detail=1"
    }' --hostnameSuffix=local &
pid=$!

sleep 1

check h "" "http://home:1234/"
check h.local "" "http://home:1234/"
check_no_redir blah.local ""

check h "/foo" "http://home:1234/foo"
check h.local "/foo" "http://home:1234/foo"
check_no_redir blah.local "/foo"

check h "/foo?key=val" "http://home:1234/foo?key=val"
check h.local "/foo?key=val" "http://home:1234/foo?key=val"

check h "/grafana" "http://grafana:2345/graphs/index"
check h.local "/grafana" "http://grafana:2345/graphs/index"

check h "/grafana/foo" "http://grafana:2345/graphs/index/foo"
check h.local "/grafana/foo" "http://grafana:2345/graphs/index/foo"

check h "/grafana/overview" "http://grafana:2345/graphs/12345"
check h.local "/grafana/overview" "http://grafana:2345/graphs/12345"

check h "/grafana/overview?key=val" "http://grafana:2345/graphs/12345?key=val"
check h.local "/grafana/overview?key=val" "http://grafana:2345/graphs/12345?key=val"

check h "/graph/1234" "http://grafana:2345/graph?id=1234&detail=1"
check h.local "/graph/1234" "http://grafana:2345/graph?id=1234&detail=1"

echo kill $pid
kill $pid 2> /dev/null


# kill all child processes
pkill -P $$

if [[ $ERRORS == 0 ]]; then
    echo PASS
else
    echo FAIL: $ERRORS
    exit 1
fi
