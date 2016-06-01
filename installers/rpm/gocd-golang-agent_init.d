#!/bin/bash
#*************************GO-LICENSE-START********************************
# Copyright 2014 ThoughtWorks, Inc.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#*************************GO-LICENSE-END**********************************

# chkconfig: 2345 90 90
# description: GoCD Golang Agent
# processname: java
### BEGIN INIT INFO
# Provides: gocd-golang-agent
# Required-Start: $network $remote_fs
# Required-Stop: $network $remote_fs
# Default-Start: 2 3 4 5
# Default-Stop: 0 1 6
# Description: Start the GoCD Golang Agent
### END INIT INFO
SERVICE_NAME=${0##*/}

# Strips initV leading chars if the script is located in rcX.d (automatically executed on runlevel change)
INIT_DIR=${0%/*}; echo ${INIT_DIR##*/} | grep -e '^rc[0-9]\.d$' >/dev/null && SERVICE_NAME=$(echo "${SERVICE_NAME}" | sed 's/^[SK][0-9][0-9]//');

PID_FILE="/var/run/${SERVICE_NAME}/${SERVICE_NAME}.pid"
CUR_USER=`whoami`

# LSB implimentation is not standard across distros, so we have to roll our own...
killproc() {
    pkill -u go -f /opt/${SERVICE_NAME}/bin/${SERVICE_NAME}
}

start_daemon() {
    eval "$@"
}

log_success_msg() {
    echo "$@"
}

log_failure_msg() {
    echo "$@"
}

. /etc/default/${SERVICE_NAME}

check_proc() {
    pgrep -u go -f /opt/${SERVICE_NAME}/bin/${SERVICE_NAME} >/dev/null    
}

start_go_agent() {
    if [ "${CUR_USER}" != "root" ] && [ "${CUR_USER}" != "go" ]; then
        log_failure_msg "Go Agent can only be started as 'root' or 'go' user."
        exit 4
    fi

    check_proc
    if [ $? -eq 0 ]; then
        log_success_msg "Go Agent already running."
        exit 0
    fi

    [ -d /var/run/${SERVICE_NAME} ] || (mkdir /var/run/${SERVICE_NAME} && chown -R go:go /var/run/${SERVICE_NAME})

    if [ "${CUR_USER}" == "root" ]; then
      start_daemon "/bin/su - go -c '/opt/${SERVICE_NAME}/bin/agent.sh'"
    else
      start_daemon /opt/${SERVICE_NAME}/bin/agent.sh ${SERVICE_NAME}
    fi

    # Sleep for a while to see if Cruise bleats about anything
    sleep 15
    check_proc

    if [ $? -eq 0 ]; then
        log_success_msg "Started GoCD Golang Agent."
    else
        log_failure_msg "Error starting GoCD Golang Agent."
        exit -1
    fi
}

stop_go_agent() {
    if [ "${CUR_USER}" != "root" ] && [ "${CUR_USER}" != "go" ]; then
        log_failure_msg "You do not have permission to stop the GoCD Golang Agent"
        exit 4
    fi

    check_proc

    if [ $? -eq 0 ]; then
        killproc -p $PID_FILE /opt/${SERVICE_NAME}/bin/agent.sh >/dev/null

        # Make sure it's dead before we return
        until [ $? -ne 0 ]; do
            sleep 1
            check_proc
        done
    
        check_proc
        if [ $? -eq 0 ]; then
            log_failure_msg "Error stopping Go Agent."
            exit -1
        else
            log_success_msg "Stopped GoCD Golang Agent."
        fi
    else
        log_failure_msg "Go is not running or you don't have permission to stop it"
    fi
}

go_status() {
    check_proc
    if [ $? -eq 0 ]; then
        log_success_msg "GoCD Golang Agent is running."
    else
        log_failure_msg "GoCD Golang Agent is stopped."
        exit 3
    fi
}

case "$1" in
    start)
        start_go_agent
        ;;
    stop)
        stop_go_agent
        ;;
    restart)
        stop_go_agent
        start_go_agent
        ;;
    status)
        go_status
        ;;
    *)
        echo "Usage: $0 {start|stop|restart|status}"
        exit 1
esac

exit 0
