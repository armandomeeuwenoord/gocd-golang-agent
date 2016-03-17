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
	"crypto/tls"
	"errors"
	"flag"
	. "github.com/gocd-contrib/gocd-golang-agent/agent"
	"github.com/gocd-contrib/gocd-golang-agent/protocal"
	"github.com/gocd-contrib/gocd-golang-agent/server"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

var (
	goServerUrl string
	goServer    *server.Server
	stateLog    *StateLog
	buildId     string
)

func TestReportStatusAndSetCookieAfterConnected(t *testing.T) {
	buildId = "TestReportStatusAndSetCookieAfterConnected"
	stateLog.Reset(buildId, AgentId)
	done := startAgent(t)
	compose := protocal.ComposeCommand(
		startCmd(),
		protocal.ReportCurrentStatusCommand("Preparing"),
		protocal.ReportCurrentStatusCommand("Building"),
		protocal.ReportCompletingCommand(),
		protocal.ReportCompletedCommand(),
		protocal.EndCommand(),
	)
	goServer.Send(AgentId, protocal.CmdMessage(compose))

	waitForNextState(t, "agent Building")
	if GetState("cookie") == "" {
		t.Fatal("cookie is not set")
	}

	waitForNextState(t, "build Preparing")
	waitForNextState(t, "build Building")
	waitForNextState(t, "build Passed")
	waitForNextState(t, "build Passed")

	waitForNextState(t, "agent Idle")
	goServer.Send(AgentId, protocal.ReregisterMessage())
	<-done
}

func TestEcho(t *testing.T) {
	buildId = "TestEcho"
	stateLog.Reset(buildId, AgentId)
	done := startAgent(t)

	compose := protocal.ComposeCommand(
		startCmd(),
		protocal.EchoCommand("echo hello world"),
		protocal.EndCommand(),
	)
	goServer.Send(AgentId, protocal.CmdMessage(compose))
	waitForNextState(t, "agent Building")
	waitForNextState(t, "agent Idle")

	log, err := goServer.ConsoleLog(buildId, AgentId)
	if err != nil {
		t.Fatal("can't get console log: ", err)
	}
	if !strings.Contains(string(log), "echo hello world") {
		t.Fatalf("console log dos not contain echo content: %v", string(log))
	}

	goServer.Send(AgentId, protocal.ReregisterMessage())
	<-done
}

func TestExport(t *testing.T) {
	buildId = "TestExport"
	stateLog.Reset(buildId, AgentId)
	done := startAgent(t)
	compose := protocal.ComposeCommand(
		startCmd(),
		protocal.ExportCommand(map[string]string{
			"env1": "value1",
			"env2": "value2",
			"env3": "value3",
		}),
		protocal.ExportCommand(nil),
		protocal.EndCommand(),
	)
	goServer.Send(AgentId, protocal.CmdMessage(compose))
	waitForNextState(t, "agent Building")
	waitForNextState(t, "agent Idle")

	log, err := goServer.ConsoleLog(buildId, AgentId)
	if err != nil {
		t.Fatal("can't get console log: ", err)
	}
	if !strings.Contains(string(log), "export env1=value1") {
		t.Fatalf("no export env1: \n%v", string(log))
	}
	if !strings.Contains(string(log), "export env2=value2") {
		t.Fatalf("no export env2: \n%v", string(log))
	}
	if !strings.Contains(string(log), "export env3=value3") {
		t.Fatalf("no export env3: \n%v", string(log))
	}

	goServer.Send(AgentId, protocal.ReregisterMessage())
	<-done
}

func TestTestCommand(t *testing.T) {
	buildId = "TestTestCommand"
	stateLog.Reset(buildId, AgentId)
	done := startAgent(t)
	_, file, _, _ := runtime.Caller(0)
	compose := protocal.ComposeCommand(
		startCmd(),
		protocal.EchoCommand("file exist").SetTest(protocal.TestCommand("-d", file)),
		protocal.EchoCommand("file not exist").SetTest(protocal.TestCommand("-d", "no"+file)),
		protocal.EndCommand(),
	)
	goServer.Send(AgentId, protocal.CmdMessage(compose))
	waitForNextState(t, "agent Building")
	waitForNextState(t, "agent Idle")

	log, err := goServer.ConsoleLog(buildId, AgentId)
	if err != nil {
		t.Fatal("can't get console log: ", err)
	}
	if !strings.Contains(string(log), "file exist") {
		t.Fatalf("should have 'file exist': \n%v", string(log))
	}

	goServer.Send(AgentId, protocal.ReregisterMessage())
	<-done
}

func TestExecCommand(t *testing.T) {
	buildId = "TestExecCommand"
	stateLog.Reset(buildId, AgentId)
	done := startAgent(t)

	compose := protocal.ComposeCommand(
		startCmd(),
		protocal.ExecCommand("echo 'echo from exec echo'"),
		protocal.EndCommand(),
	)
	goServer.Send(AgentId, protocal.CmdMessage(compose))
	waitForNextState(t, "agent Building")
	waitForNextState(t, "agent Idle")

	log, err := goServer.ConsoleLog(buildId, AgentId)
	if err != nil {
		t.Fatal("can't get console log: ", err)
	}
	if !strings.Contains(string(log), "echo from exec echo") {
		t.Fatalf("No `go help` output: %v", string(log))
	}

	goServer.Send(AgentId, protocal.ReregisterMessage())
	<-done
}

func TestRunIfConfig(t *testing.T) {
	buildId = "TestRunIfConfig"
	stateLog.Reset(buildId, AgentId)
	done := startAgent(t)

	compose := protocal.ComposeCommand(
		startCmd(),
		protocal.EchoCommand("should not echo if failed when passed").RunIf("failed"),
		protocal.EchoCommand("should echo if any when passed").RunIf("any"),
		protocal.EchoCommand("should echo if passed when passed").RunIf("passed"),
		protocal.ExecCommand("cmdnotexist"),
		protocal.EchoCommand("should echo if failed when failed").RunIf("failed"),
		protocal.EchoCommand("should echo if any when failed").RunIf("any"),
		protocal.EchoCommand("should not echo if passed when failed").RunIf("passed"),
		protocal.EndCommand(),
	)
	goServer.Send(AgentId, protocal.CmdMessage(compose))
	waitForNextState(t, "agent Building")
	waitForNextState(t, "agent Idle")

	log, err := goServer.ConsoleLog(buildId, AgentId)
	if err != nil {
		t.Fatal("can't get console log: ", err)
	}
	consoleLog := string(log)

	expected := `should echo if any when passed
should echo if passed when passed
should echo if failed when failed
should echo if any when failed`
	for _, expt := range strings.Split(expected, "\n") {
		if !strings.Contains(consoleLog, expt) {
			t.Fatalf("should contain '%v', but not, log: \n%v", expt, consoleLog)
		}
	}

	unexpected := `should not echo if failed when passed
should not echo if passed when failed`
	for _, unexpt := range strings.Split(unexpected, "\n") {
		if strings.Contains(consoleLog, unexpt) {
			t.Fatalf("should not contain '%v', log: \n%v", unexpt, consoleLog)
		}
	}

	goServer.Send(AgentId, protocal.ReregisterMessage())
	<-done
}

func TestComposeCommandWithRunIfConfig(t *testing.T) {
	buildId = "TestComposeCommand"
	stateLog.Reset(buildId, AgentId)
	done := startAgent(t)

	compose := protocal.ComposeCommand(
		startCmd(),
		protocal.ComposeCommand(
			protocal.ComposeCommand(
				protocal.EchoCommand("hello world1"),
				protocal.EchoCommand("hello world2"),
			).RunIf("any"),
			protocal.ComposeCommand(
				protocal.EchoCommand("hello world3"),
				protocal.EchoCommand("hello world4"),
			),
		).RunIf("failed"),
		protocal.ComposeCommand(
			protocal.EchoCommand("hello world5").RunIf("failed"),
			protocal.EchoCommand("hello world6"),
		),
		protocal.EndCommand(),
	)
	goServer.Send(AgentId, protocal.CmdMessage(compose))
	waitForNextState(t, "agent Building")
	waitForNextState(t, "agent Idle")

	log, err := goServer.ConsoleLog(buildId, AgentId)
	if err != nil {
		t.Fatal("can't get console log: ", err)
	}
	consoleLog := string(log)
	lines := strings.Split(consoleLog, "\n")
	if len(lines) != 2 {
		t.Fatalf("should have 2 line log: %v", consoleLog)
	}
	if !strings.Contains(lines[0], "hello world6") {
		t.Fatalf("should contain hello world6, but not: %v", consoleLog)
	}
	if "" != lines[1] {
		t.Fatalf("unexpected log: '%v'", lines[1])
	}

	goServer.Send(AgentId, protocal.ReregisterMessage())
	<-done
}

func startAgent(t *testing.T) chan bool {
	done := make(chan bool)
	go func() {
		t.Log("start agent")
		err := Start()
		if err.Error() != "received reregister message" {
			t.Error("Unexpected error to quit agent: ", err)
		}
		close(done)
	}()
	waitForNextState(t, "agent Idle")
	return done
}

func waitForNextState(t *testing.T, expected string) {
	state := stateLog.Next()
	if expected != state {
		_, file, line, _ := runtime.Caller(1)
		finfo, _ := os.Stat(file)
		t.Fatalf("expected agent state: %v, but get: %v\n%v:%v:", expected, state, finfo.Name(), line)
	}
}

func startCmd() *protocal.BuildCommand {
	return protocal.StartCommand(goServer.BuildContext(buildId))
}

func TestMain(m *testing.M) {
	flag.Parse()

	workingDir, err := ioutil.TempDir("", "gocd-golang-agent")
	if err != nil {
		panic(err)
	}
	println("working directories:", workingDir)
	serverWorkingDir := filepath.Join(workingDir, "server")
	agentWorkingDir := filepath.Join(workingDir, "agent")

	err = os.MkdirAll(serverWorkingDir, 0777)
	if err != nil {
		panic(err)
	}
	err = os.MkdirAll(agentWorkingDir, 0777)
	if err != nil {
		panic(err)
	}

	startServer(serverWorkingDir)
	os.Setenv("DEBUG", "t")
	os.Setenv("GOCD_SERVER_URL", goServerUrl)
	os.Setenv("GOCD_SERVER_WEB_SOCKET_PATH", server.WebSocketPath)
	os.Setenv("GOCD_SERVER_REGISTRATION_PATH", server.RegistrationPath)
	os.Setenv("GOCD_AGENT_WORK_DIR", agentWorkingDir)
	os.Setenv("GOCD_AGENT_LOG_DIR", agentWorkingDir)

	Initialize()

	os.Exit(m.Run())
}

func startServer(workingDir string) {
	certFile := filepath.Join(workingDir, "cert.pem")
	keyFile := filepath.Join(workingDir, "private.pem")
	cert := server.NewCert()
	err := cert.Generate(certFile, keyFile)
	if err != nil {
		panic(err)
	}
	port := "1234"
	stateLog = &StateLog{states: make(chan string)}
	goServerUrl = "https://" + cert.Host + ":" + port
	goServer = server.New(port,
		certFile,
		keyFile,
		workingDir,
		MakeLogger(workingDir, "server.log", true).Info)
	goServer.StateListeners = []server.StateListener{stateLog}

	go func() {
		e := goServer.Start()
		panic(e.Error())
	}()

	println("wait for server started")
	if err := waitForServerStarted(goServerUrl + server.StatusPath); err != nil {
		panic(err)
	}
	println("server started")
}

func waitForServerStarted(url string) error {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}
	timeout := time.After(5 * time.Second)
	for {
		select {
		case <-timeout:
			return errors.New("wait for server start timeout")
		default:
			_, err := client.Get(url)
			if err == nil {
				return nil
			}
		}
	}
}

type StateLog struct {
	states           chan string
	mu               sync.Mutex
	buildId, agentId string
}

func (log *StateLog) Notify(class, id, state string) {
	log.mu.Lock()
	defer log.mu.Unlock()
	switch class {
	case "agent":
		if id == log.agentId {
			log.states <- "agent " + state
		}
	case "build":
		if id == log.buildId {
			log.states <- "build " + state
		}
	}
}

func (log *StateLog) Next() string {
	select {
	case state := <-log.states:
		return state
	case <-time.After(5 * time.Second):
		return "timeout"
	}
}

func (log *StateLog) Reset(buildId, agentId string) {
	log.mu.Lock()
	defer log.mu.Unlock()
	log.buildId = buildId
	log.agentId = agentId
}