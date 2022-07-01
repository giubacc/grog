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
	"context"
	"fmt"
	"grog/util"
	"net"
	"strings"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
	"golang.org/x/net/ipv4"
)

type MCastHelper struct {
	//config
	Cfg *util.Config

	//Status
	Status uint

	//logger
	logger *logrus.Logger

	//Node ID
	NodeID uint64

	//chosen inet
	Inet net.Interface

	//incoming multicast connection
	iPktConn  net.PacketConn
	iNPktConn *ipv4.PacketConn

	//outgoing multicast info
	outgPktUDPAddr net.UDPAddr

	//channels used to receive Alive messages (UDP)
	AliveChanIncoming chan []byte

	//channels used to receive Incremental messages (UDP)
	IncrementalChanIncoming chan []byte

	//channels used to send UPD messages
	MCastChanOutgoing chan []byte

	//channel used for generic incoming requests
	RequestChan chan util.Request
}

func (m *MCastHelper) WaitForStatus(status uint) error {
	statusReq := util.Request{ReqCode: util.STATUS, Response: make(chan interface{})}
	m.RequestChan <- statusReq
	interrupter := time.NewTicker(time.Millisecond * time.Duration(m.Cfg.LoopReactivity))
	defer interrupter.Stop()

out:
	for {
		select {
		case res := <-statusReq.Response:
			if res.(uint) == status {
				m.logger.Tracef("status reached:%d", status)
				break out
			}
		case <-interrupter.C:
			m.RequestChan <- statusReq
		}
	}
	return nil
}

func (m *MCastHelper) Init() error {
	//logger init
	m.logger = util.GetLogger("mcst", m.Cfg)

	//make request channel
	m.RequestChan = make(chan util.Request)

	m.Status = util.INIT
	return nil
}

func (m *MCastHelper) establish_multicast() error {
	m.logger.Tracef("establishing multicast: group:%s - port:%d", m.Cfg.MulticastAddress, m.Cfg.MulticastPort)

	config := &net.ListenConfig{Control: mcastIncoRawConnCfg}

	if iPktConn, err := config.ListenPacket(context.Background(), "udp4", fmt.Sprintf("0.0.0.0:%d", m.Cfg.MulticastPort)); err != nil {
		m.logger.Errorf("listening packet:%s", err.Error())
		return err
	} else {
		m.iPktConn = iPktConn
	}

	m.iNPktConn = ipv4.NewPacketConn(m.iPktConn)
	mgroup := net.ParseIP(m.Cfg.MulticastAddress)
	m.outgPktUDPAddr = net.UDPAddr{IP: mgroup, Port: int(m.Cfg.MulticastPort)}

	if err := m.iNPktConn.JoinGroup(&m.Inet, &m.outgPktUDPAddr); err != nil {
		m.logger.Errorf("joining multicast group:%s", err.Error())
		return err
	}

	if err := m.iNPktConn.SetControlMessage(ipv4.FlagSrc, true); err != nil {
		m.logger.Errorf("SetControlMessage:%s", err.Error())
		return err
	}
	if err := m.iNPktConn.SetControlMessage(ipv4.FlagDst, true); err != nil {
		m.logger.Errorf("SetControlMessage:%s", err.Error())
		return err
	}

	return nil
}

func mcastIncoRawConnCfg(network, address string, conn syscall.RawConn) error {
	return conn.Control(func(descriptor uintptr) {
		if err := syscall.SetsockoptInt(int(descriptor), syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1); err != nil {
			logrus.StandardLogger().Errorf("SetsockoptInt:SO_REUSEADDR - %s", err.Error())
		}
	})
}

func (m *MCastHelper) mcastSender() {
	for {
		buff := <-m.MCastChanOutgoing
		if nsent, err := m.iNPktConn.WriteTo(buff, nil, &m.outgPktUDPAddr); err != nil {
			m.logger.Errorf("WriteTo:%s", err.Error())
		} else {
			m.logger.Tracef("[UDP] sent %d bytes to:%s", nsent, m.outgPktUDPAddr.String())
		}
	}
}

func (m *MCastHelper) mcastReader() {
	buff := make([]byte, 1500)
	//reading loop from multicast connection
	for {
		nread, cm, _, err := m.iNPktConn.ReadFrom(buff)
		if err != nil {
			m.logger.Errorf("ReadFrom:%s", err.Error())
		} else {
			m.logger.Tracef("[UDP] read %d bytes from:%s", nread, cm.String())

			sliced_buff := buff[0:nread]
			raw_msg := string(sliced_buff)
			if m.Cfg.VerbLevel >= util.VL_TRACE {
				m.logger.Tracef("%s", string(raw_msg))
			}

			if idx0 := strings.Index(raw_msg, ":"); idx0 != -1 {
				if idx1 := strings.Index(raw_msg[idx0:], "\""); idx1 != -1 {
					msgType := raw_msg[idx0:][idx1+1]
					if m.Cfg.VerbLevel >= util.VL_TRACE {
						m.logger.WithFields(logrus.Fields{
							"type": msgType,
							"msg":  string(sliced_buff),
						}).Trace("<<")
					}
					switch msgType {
					case 'A':
						m.AliveChanIncoming <- sliced_buff
					case 'I':
						m.IncrementalChanIncoming <- sliced_buff
					}
				}
			}
		}
	}
}

func (m *MCastHelper) processRequest(request util.Request) error {
	switch request.ReqCode {
	case util.STATUS:
		request.Response <- m.Status
	}
	return nil
}

func (m *MCastHelper) processEvents() error {
	m.Status = util.LOOP
	for {
		req := <-m.RequestChan
		m.processRequest(req)
	}
}

func (m *MCastHelper) Run() error {
	if err := m.establish_multicast(); err != nil {
		return err
	}

	//start mcastSender
	go m.mcastSender()
	//start mcastReader
	go m.mcastReader()

	return m.processEvents()
}
