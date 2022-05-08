#!/usr/bin/env bash

trap "rm server; kill 0" EXIT   # 在shell脚本退出时删掉临时文件，结束子进程

go build -o server
./server -port = 8081 &
./server -port = 8082 &
./server -port=8003 -api=1 &

sleep 2
echo ">>> start test"
curl "http://localhost:9999/api?key=Tom" &
curl "http://localhost:9999/api?key=Tom" &
curl "http://localhost:9999/api?key=Tom" &

wait