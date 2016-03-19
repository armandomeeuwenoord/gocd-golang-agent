/*
 * Copyright 2016 ThoughtWorks, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package agent_test

import (
	. "github.com/gocd-contrib/gocd-golang-agent/agent"
	"github.com/gocd-contrib/gocd-golang-agent/protocal"
	"github.com/xli/assert"
	"runtime"
	"testing"
)

func TestTestCommand(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)

	setUp(t)
	defer tearDown()

	goServer.SendBuild(AgentId, buildId,
		protocal.EchoCommand("file exist").SetTest(protocal.TestCommand("-d", file)),
		protocal.EchoCommand("file not exist").SetTest(protocal.TestCommand("-d", "no"+file)),
	)

	assert.Equal(t, "agent Building", stateLog.Next())
	assert.Equal(t, "agent Idle", stateLog.Next())

	log, err := goServer.ConsoleLog(buildId)
	assert.Nil(t, err)
	assert.Equal(t, "file exist\n", trimTimestamp(log))
}