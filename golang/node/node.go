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

package node

import (
	"encoding/json"
	"fmt"
	"grog/network"
	"grog/util"
	"net"
	"time"

	"github.com/sirupsen/logrus"
)

type Node struct {
	//configuration
	Cfg util.Config

	//Node ID
	NodeID uint64

	//the last sequence number produced by the node
	SeqNo uint64

	//chosen inet
	Inet net.Interface

	//TCP acceptor
	acceptor network.Acceptor

	//TCP streamer
	streamer network.Streamer

	//multicast helper
	mcastHelper network.MCastHelper

	//channel used to serve incoming TCP connections
	EnteringChan chan net.Conn

	//channels used to receive Alive messages (UDP)
	AliveChanIncoming chan []byte

	//channels used to receive Incremental messages (UDP)
	IncrementalChanIncoming chan []byte

	//channels used to send UPD messages
	MCastChanOutgoing chan []byte

	//Map maintainer
	mapMaintainer MapMaintainer

	//logger
	logger *logrus.Logger

	//a map containing the last sequence number produced by a node [nid:seqno]
	NodeSeqNo map[uint64]uint64

	//CLI
	OpDone bool
}

func (p *Node) Run() error {
	if err := p.init(); err != nil {
		return err
	}

	if err := p.start(); err != nil {
		return err
	}

	p.bcastAliveMessage()
	return p.processEvents()
}

func (n *Node) choose_inet() error {
	//enum net interfaces, choose the one specified in config
	if nis, err := net.Interfaces(); err != nil {
		return err
	} else {
		var chosenIntf = false
		for _, ni := range nis {
			addr, _ := ni.Addrs()
			if len(addr) > 0 {
				addr0 := addr[0].String()
				n.logger.Tracef("inspecting host-intf:%s-%s ...", ni.Name, addr0)

				if !chosenIntf && ni.Name == n.Cfg.NetInterfaceName {
					n.Inet = ni
					n.logger.Tracef("chosen host-intf:%s-%s", ni.Name, addr0)
					chosenIntf = true
					break
				}
			}
		}
		if !chosenIntf {
			return &util.GrogError{Code: util.RetCode_BADCFG}
		}
	}
	return nil
}

func (n *Node) init() error {
	//logger init
	n.logger = util.GetLogger("node", &n.Cfg)

	if err := n.choose_inet(); err != nil {
		return err
	}

	n.NodeID = uint64(time.Now().UnixNano())
	n.logger.Debugf("NID:%d", n.NodeID)

	n.EnteringChan = make(chan net.Conn)
	n.AliveChanIncoming = make(chan []byte)
	n.IncrementalChanIncoming = make(chan []byte)
	n.MCastChanOutgoing = make(chan []byte)

	//TCP acceptor
	n.acceptor.Cfg = &n.Cfg
	n.acceptor.Inet = n.Inet
	n.acceptor.EnteringChan = n.EnteringChan

	//TCP streamer
	n.streamer.Cfg = &n.Cfg

	//multicast helper
	n.mcastHelper.Cfg = &n.Cfg
	n.mcastHelper.NodeID = n.NodeID
	n.mcastHelper.Inet = n.Inet
	n.mcastHelper.AliveChanIncoming = n.AliveChanIncoming
	n.mcastHelper.IncrementalChanIncoming = n.IncrementalChanIncoming
	n.mcastHelper.MCastChanOutgoing = n.MCastChanOutgoing

	//map maintainer
	n.mapMaintainer.Cfg = &n.Cfg
	n.mapMaintainer.NodeID = n.NodeID

	return nil
}

func (n *Node) doCLIOp() {
	ns := "{empty}"
	if n.Cfg.Namespace != "" {
		ns = n.Cfg.Namespace
	}

	switch n.Cfg.Op {
	case util.OP_GET:
		if val := n.mapMaintainer.Get(n.Cfg.Namespace, n.Cfg.Key); val != nil {
			fmt.Printf("%s\n", *val)
		} else {
			fmt.Printf("no key=%s defined in %s namespace\n", n.Cfg.Key, ns)
		}
	case util.OP_SET:
		if incrMsg, err := n.mapMaintainer.Set(n.Cfg.Namespace, n.Cfg.Key, n.Cfg.Val); err == nil {
			if err := n.bcastIncrementalMessage(*incrMsg); err != nil {
				n.logger.Errorf("broadcasting incremental:%s", err.Error())
			}
			fmt.Printf("updated key=%s in %s namespace\n", n.Cfg.Key, ns)
		}
	case util.OP_DEL:
		if incrMsg, err := n.mapMaintainer.Del(n.Cfg.Namespace, n.Cfg.Key); err == nil && incrMsg != nil {
			if err := n.bcastIncrementalMessage(*incrMsg); err != nil {
				n.logger.Errorf("broadcasting incremental:%s", err.Error())
			}
		}
		fmt.Printf("deleted key=%s in %s namespace\n", n.Cfg.Key, ns)
	}

	n.OpDone = true
}

func (n *Node) start() error {
	n.logger.Tracef("initializing map maintainer ...")

	if err := n.mapMaintainer.Init(); err != nil {
		return err
	}

	n.logger.Tracef("initializing streamer ...")

	if err := n.streamer.Init(); err != nil {
		return err
	}

	n.logger.Tracef("starting acceptor ...")

	if err := n.acceptor.Init(); err != nil {
		return err
	}
	go n.acceptor.Run()
	n.acceptor.WaitForStatus(util.LOOP)

	n.logger.Tracef("starting multicast ...")

	if err := n.mcastHelper.Init(); err != nil {
		return err
	}
	go n.mcastHelper.Run()
	n.mcastHelper.WaitForStatus(util.LOOP)

	return nil
}

func (n *Node) processEvents() error {
	n.logger.Tracef("start processing events ...")

	interrupter := time.NewTicker(time.Millisecond * time.Duration(n.Cfg.LoopReactivity))
	defer interrupter.Stop()

	aliveTicker := time.NewTicker(time.Second * 20)
	defer aliveTicker.Stop()

out:
	for {
		select {
		case <-interrupter.C:
			if err := n.processStatus(); err != nil && err.(*util.GrogError).Code == util.RetCode_EXIT {
				break out
			}
			if n.mapMaintainer.Status == util.SYNC {
				n.mapMaintainer.ProcessStatusSynch()
			} else if !n.OpDone && n.mapMaintainer.Status == util.LOOP {
				n.doCLIOp()
			}
		case <-aliveTicker.C:
			n.bcastAliveMessage()
		case conn := <-n.EnteringChan:
			n.sendSnapshotMessage(conn)
		case pkt := <-n.AliveChanIncoming:
			if err := n.processAliveMsg(pkt); err != nil {
				n.logger.Errorf("processAliveMsg:%s", err.Error())
			}
		case pkt := <-n.IncrementalChanIncoming:
			if err := n.processIncrementalMsg(pkt); err != nil {
				n.logger.Errorf("processIncrementalMsg:%s", err.Error())
			}
		}
	}

	n.logger.Tracef("stop processing events")
	return nil
}

func (n *Node) processStatus() error {

	if !n.Cfg.Daemonize {
		if n.Cfg.Op != util.OP_NOP {
			if n.OpDone {
				return &util.GrogError{Code: util.RetCode_EXIT}
			}
		} else {
			return &util.GrogError{Code: util.RetCode_EXIT}
		}
	}

	return nil
}

func (n *Node) processAliveMsg(pkt []byte) error {
	msg := util.AliveMsg{}
	if err := json.Unmarshal(pkt, &msg); err != nil {
		n.logger.Errorf("unmarshalling alive:%s", err.Error())
		return err
	}

	if msg.Nid == n.NodeID {
		n.logger.Tracef("alive message from this node, discarding ...")
		return nil
	}

	if n.mapMaintainer.Ts == 0 {
		if msg.Ts == 0 {
			n.logger.Tracef("alive message from synching node: this node is synching, discarding ...")
		} else {
			n.logger.Tracef("alive message from looping node: this node is synching, asking for snapshot ...")
			if buff, err := n.streamer.DialAndReceiveSnapshotBuffer(msg.Address); err != nil {
				n.logger.Errorf("receiving snapshot:%s", err.Error())
			} else {
				msg := util.SnapshotMsg{}
				if err := json.Unmarshal(buff, &msg); err != nil {
					n.logger.Errorf("unmarshalling snapshot :%s", err.Error())
				} else {
					n.mapMaintainer.OfferSnapshot(msg)
				}
			}
		}
	} else {
		if msg.Ts == 0 {
			n.logger.Tracef("alive message from synching node: this node is looping, notifying ...")
			n.bcastAliveMessage()
		} else {
			//this node is looping, here we should check sequence numbers
		}
	}

	return nil
}

func (n *Node) processIncrementalMsg(pkt []byte) error {
	msg := util.IncrementalMsg{}
	if err := json.Unmarshal(pkt, &msg); err != nil {
		n.logger.Errorf("unmarshalling incremental:%s", err.Error())
		return err
	}

	if msg.Nid == n.NodeID {
		n.logger.Tracef("incremental message from this node, discarding ...")
		return nil
	}

	if n.mapMaintainer.Ts == 0 {
		n.logger.Tracef("incremental message: this node is synching, discarding ...")
	} else {
		n.mapMaintainer.OfferIncremental(msg)
	}

	return nil
}

func (n *Node) bcastAliveMessage() error {
	msg := util.AliveMsg{
		Type:    util.MsgTypeAlive,
		Ts:      n.mapMaintainer.Ts,
		Nid:     n.NodeID,
		SeqNo:   n.SeqNo,
		Address: n.acceptor.Listener.Addr().String()}

	if buff, err := json.Marshal(msg); err != nil {
		n.logger.Errorf("building alive msg:%s", err.Error())
		return err
	} else {
		if n.Cfg.VerbLevel >= util.VL_TRACE {
			n.logger.WithFields(logrus.Fields{
				"type": "A",
				"msg":  string(buff),
			}).Trace(">>")
		}
		n.MCastChanOutgoing <- buff
	}
	return nil
}

func (n *Node) sendSnapshotMessage(conn net.Conn) error {
	if msg, err := n.mapMaintainer.TakeSnapshot(); err == nil {
		if buff, err := json.Marshal(*msg); err != nil {
			n.logger.Errorf("marshalling snapshot msg:%s", err.Error())
			return err
		} else {
			if n.Cfg.VerbLevel >= util.VL_TRACE {
				n.logger.WithFields(logrus.Fields{
					"type": "S",
					"msg":  string(buff),
				}).Trace(">>")
			}
			go n.streamer.SendSnapshotBuffer(conn, buff)
		}
	}
	return nil
}

func (n *Node) bcastIncrementalMessage(msg util.IncrementalMsg) error {
	n.SeqNo++
	if buff, err := json.Marshal(msg); err != nil {
		n.logger.Errorf("marshalling incremental msg:%s", err.Error())
		return err
	} else {
		if n.Cfg.VerbLevel >= util.VL_TRACE {
			n.logger.WithFields(logrus.Fields{
				"type": "I",
				"msg":  string(buff),
			}).Trace(">>")
		}
		n.MCastChanOutgoing <- buff
	}
	return nil
}
