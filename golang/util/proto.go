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

package util

const (
	OP_NOP = iota
	OP_GET
	OP_SET
	OP_DEL
)

const (
	Type     = "type"     //message type
	Ts       = "ts"       //map's timestamp at the sending epoch of the message
	Nid      = "nid"      //node-id, it is set at the starting epoch of the node
	Seqno    = "seqno"    //last sequence number produced by the node, it is incremented every time an I message is produced
	Seqnos   = "seqnos"   //array containing all the sequence numbers of every node contributed updating the map
	Address  = "address"  //node's ip:port where it listens for snapshot requests
	Snapshot = "snapshot" //map's snapshot data
	Ns       = "ns"       //item namespace
)

const (
	MsgTypeAlive       = "A" //A (alive)
	MsgTypeSnapshot    = "S" //S (snapshot)
	MsgTypeIncremental = "I" //I (incremental)
)

/**
 * Alive message (UDP multicast):
 */
type AliveMsg struct {
	Type    string `json:"type"`
	Ts      uint64 `json:"ts"`      //map's timestamp at the sending epoch of the message
	Nid     uint64 `json:"nid"`     //node-id
	Seqno   uint64 `json:"seqno"`   //the last sequence number produced by this node
	Address string `json:"address"` //the node's ip:port where it listens for snapshot requests
}

/**
 * An association between a nid and its last produced seqno.
 * It is used inside the Snapshot message.
 */
type NidSeqno struct {
	Nid   uint64 `json:"nid"`
	Seqno uint64 `json:"seqno"`
}

/**
 * Single snapshot's Key-Value
 */
type SnapshotItem struct {
	Ts  uint64 `json:"ts"`  //associated event's timestamp
	Nid uint64 `json:"nid"` //associated event's node-id
	Key string `json:"key"` //item's key
	Val string `json:"val"` //item's value
}

/**
 * Namespaced context for an array of SnapshotItems
 */
type SnapshotNs struct {
	Ns            string `json:"ns"` //item's namespace
	SnapshotItems []SnapshotItem
}

/**
 * Snapshot field
 */
type SnapshotFld struct {
	Ts          uint64       `json:"ts"`          //map's timestamp, it's the last event timestamp
	SnapshotNss []SnapshotNs `json:"snapshot-ns"` //namespaced items
}

/**
 * Snapshot message (TCP):
 */
type SnapshotMsg struct {
	Type     string      `json:"type"`
	Ts       uint64      `json:"ts"`       //map's timestamp at the sending epoch of the message
	Nid      uint64      `json:"nid"`      //node-id
	Seqnos   []NidSeqno  `json:"seqnos"`   //array containing all the sequence numbers of every node contributed updating the map
	Snapshot SnapshotFld `json:"snapshot"` //map's snapshot
}

/**
 * Incremental message (UDP multicast):
 */
type IncrementalMsg struct {
	Type  string `json:"type"`
	Ts    uint64 `json:"ts"`    //I timestamp, it denotes the timestamp of this event
	Nid   uint64 `json:"nid"`   //node-id
	Seqno uint64 `json:"seqno"` //last sequence number produced by this node
	Op    uint16 `json:"op"`    //operation associated with the message
	Ns    string `json:"ns"`    //item's namespace
	Key   string `json:"key"`   //item's key
	Val   string `json:"val"`   //item's value
}
