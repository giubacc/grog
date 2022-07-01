/*
Copyright (c) 2022 Giuseppe Baccini - giuseppe.baccini@suse.com

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package network

import (
	"fmt"
	"grog/util"
	"net"
	"time"

	"github.com/sirupsen/logrus"
)

type Acceptor struct {
	//config
	Cfg *util.Config

	//Status
	Status uint

	//chosen inet
	Inet net.Interface

	//listening port
	ListenPort uint

	//Listener
	Listener net.Listener

	//channel used to serve incoming TCP connections
	EnteringChan chan net.Conn

	//logger
	logger *logrus.Logger

	//channel used for generic incoming requests
	RequestChan chan util.Request
}

func (a *Acceptor) Run() error {
	go a.accept()

	a.processEvents()
	return a.stop()
}

func (a *Acceptor) WaitForStatus(status uint) error {
	statusReq := util.Request{ReqCode: util.STATUS, Response: make(chan interface{})}
	a.RequestChan <- statusReq
	interrupter := time.NewTicker(time.Millisecond * time.Duration(a.Cfg.LoopReactivity))
	defer interrupter.Stop()

out:
	for {
		select {
		case res := <-statusReq.Response:
			if res.(uint) == status {
				a.logger.Tracef("status reached:%d", status)
				break out
			}
		case <-interrupter.C:
			a.RequestChan <- statusReq
		}
	}
	return nil
}

func (a *Acceptor) Init() error {
	a.ListenPort = a.Cfg.ListeningPort

	//logger init
	a.logger = util.GetLogger("acpt", a.Cfg)

	//make request channel
	a.RequestChan = make(chan util.Request)

	a.Status = util.INIT
	return nil
}

func (a *Acceptor) stop() error {
	return a.Listener.Close()
}

func (a *Acceptor) accept() error {
	for {
		if addrs, err := a.Inet.Addrs(); err == nil {
			if a.Listener, err = net.Listen("tcp", fmt.Sprintf("%s:%d", addrs[0].(*net.IPNet).IP.To4(), a.ListenPort)); err == nil {
				break
			} else {
				a.ListenPort++
				a.logger.Tracef("err:%s, try auto-adjusting listening port to:%d ...", err.Error(), a.ListenPort)
			}
		} else {
			a.logger.Errorf("err:%s, listening ...", err.Error())
			return err
		}
	}

	a.logger.Tracef("accepting ...")
	a.Status = util.LOOP

	for {
		if conn, err := a.Listener.Accept(); err != nil {
			a.logger.Errorf("err:%s, accepting connection ...", err.Error())
		} else {
			a.logger.Tracef("connection accepted")
			a.EnteringChan <- conn
		}
	}
}

func (a *Acceptor) processRequest(request util.Request) error {
	switch request.ReqCode {
	case util.STATUS:
		request.Response <- a.Status
	}
	return nil
}

func (a *Acceptor) processEvents() error {
	for {
		req := <-a.RequestChan
		a.processRequest(req)
	}
}
