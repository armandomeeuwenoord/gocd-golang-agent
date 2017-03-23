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

package agent

import (
	"github.com/gocd-contrib/gocd-golang-agent/protocol"
	"golang.org/x/net/websocket"
	"time"
)

type WebsocketConnection struct {
	Conn     *websocket.Conn
	Send     chan *protocol.Message
	Received chan *protocol.Message
}

func (wc *WebsocketConnection) Close() {
	close(wc.Send)
	err := wc.Conn.Close()
	if err != nil {
		logger.Error.Printf("Close websocket connection failed: %v", err)
	}
}

func MakeWebsocketConnection(wsLoc, httpLoc string) (*WebsocketConnection, error) {
	tlsConfig, err := GoServerTlsConfig(true)
	if err != nil {
		return nil, err
	}
	wsConfig, err := websocket.NewConfig(wsLoc, httpLoc)
	if err != nil {
		return nil, err
	}
	wsConfig.TlsConfig = tlsConfig
	LogInfo("connect to: %v", wsLoc)
	ws, err := websocket.DialConfig(wsConfig)
	if err != nil {
		return nil, err
	}
	acknowledge := make(chan string)
	send := make(chan *protocol.Message)
	received := make(chan *protocol.Message)

	go startReceiveMessage(ws, received, acknowledge)
	go startSendMessage(ws, send, acknowledge)
	return &WebsocketConnection{Conn: ws, Send: send, Received: received}, nil
}

func startSendMessage(ws *websocket.Conn, send chan *protocol.Message, acknowledge chan string) {
	defer LogDebug("! exit goroutine: send message")
	connClosed := false
loop:
	select {
	case id := <-acknowledge:
		LogInfo("Ignore acknowledge with id: %v", id)
	case msg, ok := <-send:
		if !ok {
			return
		}
		LogInfo("--> %v", msg.Action)
		if connClosed {
			logger.Error.Printf("send message failed: connection is closed")
			goto loop
		}
		if err := protocol.SendMessage(ws, msg); err == nil {
			waitForMessageAcknowledge(msg.AcknowledgeId, acknowledge)
			goto loop
		} else {
			logger.Error.Printf("send message failed: %v", err)
			if err := ws.Close(); err == nil {
				connClosed = true
			} else {
				logger.Error.Printf("Close websocket connection failed: %v", err)
			}
		}
	}
	goto loop
}

func waitForMessageAcknowledge(acknowledgeId string, acknowledge chan string) {
	for {
		select {
		case <-time.After(config.SendMessageTimeout):
			LogInfo("wait for message acknowledge timeout, id: %v", acknowledgeId)
			return
		case id := <-acknowledge:
			if id == acknowledgeId {
				return
			} else {
				LogInfo("ignore acknowledge with id: %v, expected: %v", id, acknowledgeId)
			}
		}
	}
}

func startReceiveMessage(ws *websocket.Conn, received chan *protocol.Message, acknowledge chan string) {
	defer LogDebug("! exit goroutine: receive message")
	defer close(received)
	for {
		msg, err := protocol.ReceiveMessage(ws)
		if err != nil {
			logger.Error.Printf("receive message failed: %v", err)
			return
		}
		LogInfo("<-- %v", msg.Action)

		if msg.Action == "acknowledge" {
			acknowledge <- msg.DataString()
		} else {
			received <- msg
		}
	}
}
