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
	"grog/util"
	"time"

	"github.com/sirupsen/logrus"
)

type MapItem struct {
	Ts  uint64 //associated item's timestamp
	Nid uint64 //associated item's node-id
	Val string //item's value
}

type MapMaintainer struct {
	//config
	Cfg *util.Config

	//Status
	Status uint

	//The time point this node will generate itself the timestamp;
	//This will happen if no other node will respond to initial alive sent by this node.
	//Not daemon nodes ("pure" setter or getter nodes) will shutdown at this time point.
	TpInitialSynchWindow time.Time

	//logger
	logger *logrus.Logger

	//Node ID
	NodeID uint64

	//Map's timestamp
	Ts uint64

	/*
	   The map maintained by this maintainer.
	   [namespace : [key : *MapItem]]
	*/
	Map map[string]map[string]*MapItem
}

func (m *MapMaintainer) Init() error {
	//logger init
	m.logger = util.GetLogger("mmtr", m.Cfg)

	//seconds before this node will auto generate the timestamp
	m.TpInitialSynchWindow = time.Now().Add(time.Second * time.Duration(m.Cfg.NodeSynchDuration))

	//make the Map
	m.Map = make(map[string]map[string]*MapItem)

	m.Status = util.SYNC
	return nil
}

func (m *MapMaintainer) genTS() {
	m.Ts = uint64(time.Now().UnixNano())
}

func (m *MapMaintainer) ProcessStatusSynch() error {
	now := time.Now()

	//if no other node has still responded to initial alive,
	//we generate the map's timestamp
	if m.Ts == 0 && now.After(m.TpInitialSynchWindow) {
		m.genTS()
		m.logger.Tracef("auto-generated map's timestamp: %d", m.Ts)
		m.Status = util.LOOP
	}

	return nil
}

func (m *MapMaintainer) Get(namespace string, key string) *string {
	if nsMap := m.Map[namespace]; nsMap != nil {
		if mapItem := nsMap[key]; mapItem != nil {
			return &mapItem.Val
		}
	}
	return nil
}

func (m *MapMaintainer) Set(namespace string, key string, val string) (*util.IncrementalMsg, error) {
	mapItem := &MapItem{
		Ts:  uint64(time.Now().UnixNano()),
		Nid: m.NodeID,
		Val: val}

	if nsMap := m.Map[namespace]; nsMap == nil {
		nsMap = make(map[string]*MapItem)
		nsMap[key] = mapItem
		m.Map[namespace] = nsMap
	} else {
		nsMap[key] = mapItem
	}

	incr := util.IncrementalMsg{
		Type:  util.MsgTypeIncremental,
		Ts:    mapItem.Ts,
		Nid:   m.NodeID,
		SeqNo: 0,
		Op:    util.OP_SET,
		Ns:    namespace,
		Key:   key,
		Val:   val}

	return &incr, nil
}

func (m *MapMaintainer) Del(namespace string, key string) (*util.IncrementalMsg, error) {
	if nsMap := m.Map[namespace]; nsMap != nil {
		if mapItem := nsMap[key]; mapItem != nil {

			incr := util.IncrementalMsg{
				Type:  util.MsgTypeIncremental,
				Ts:    uint64(time.Now().UnixNano()),
				Nid:   m.NodeID,
				SeqNo: 0,
				Op:    util.OP_DEL,
				Ns:    namespace,
				Key:   key}

			delete(nsMap, key)
			return &incr, nil
		}
	}
	return nil, nil
}

func (m *MapMaintainer) TakeSnapshot() (*util.SnapshotMsg, error) {
	snap := util.SnapshotMsg{Type: util.MsgTypeSnapshot, Ts: m.Ts, Nid: m.NodeID}
	snap.Snapshot.Ts = m.Ts

	for namespace, nsMap := range m.Map {
		snapNs := util.SnapshotNs{Ns: namespace}
		for key, mapItem := range nsMap {
			snapNs.SnapshotItems = append(snapNs.SnapshotItems, util.SnapshotItem{
				Ts:  mapItem.Ts,
				Nid: mapItem.Nid,
				Key: key,
				Val: mapItem.Val})
		}
		snap.Snapshot.SnapshotNss = append(snap.Snapshot.SnapshotNss, snapNs)
	}

	return &snap, nil
}

func (m *MapMaintainer) OfferSnapshot(snapshot util.SnapshotMsg) error {
	//clearing the existing map
	m.Map = make(map[string]map[string]*MapItem)

	m.Ts = snapshot.Ts
	for _, snapshotNs := range snapshot.Snapshot.SnapshotNss {
		nsMap := make(map[string]*MapItem)
		for _, snapshotItem := range snapshotNs.SnapshotItems {
			nsMap[snapshotItem.Key] = &MapItem{
				Ts:  snapshotItem.Ts,
				Nid: snapshotItem.Nid,
				Val: snapshotItem.Val}
		}
		m.Map[snapshotNs.Ns] = nsMap
	}

	m.Status = util.LOOP
	return nil
}

func (m *MapMaintainer) doSetIncr(incremental util.IncrementalMsg) error {
	mapItem := &MapItem{
		Ts:  incremental.Ts,
		Nid: incremental.Nid,
		Val: incremental.Val}

	if nsMap := m.Map[incremental.Ns]; nsMap == nil {
		nsMap = make(map[string]*MapItem)
		nsMap[incremental.Key] = mapItem
		m.Map[incremental.Ns] = nsMap
	} else {
		nsMap[incremental.Key] = mapItem
	}
	return nil
}

func (m *MapMaintainer) doDelIncr(incremental util.IncrementalMsg) error {
	if nsMap := m.Map[incremental.Ns]; nsMap != nil {
		delete(nsMap, incremental.Key)
	}
	return nil
}

func (m *MapMaintainer) OfferIncremental(incremental util.IncrementalMsg) error {
	m.Ts = incremental.Ts
	switch incremental.Op {
	case util.OP_SET:
		m.doSetIncr(incremental)
	case util.OP_DEL:
		m.doDelIncr(incremental)
	}

	return nil
}
