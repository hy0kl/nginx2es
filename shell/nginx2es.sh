#!/usr/bin/env bash
# @describe: 重启 nginx2es

#set -x
pid=$(ps axu | grep nginx2es | grep -v grep | awk '{print $2}')
if [ "x$pid" != "x" ]
then
    kill -9 "$pid"
else
    echo "nginx2es it does not working"
fi

#cd /work/op-tools &&  nohup ./nginx2es > /dev/null 2>/work/logs/nginx2es.log &

exit 0
# vim:set ts=4 sw=4 et fdm=marker:

