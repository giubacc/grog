# grog

![image](assets/grog.png)

**grog** is a cluster's protocol implementing a *namespaced distributed map*.

grog's name originates from the pirates's favorite beverage in the saga
of [Monkey Island](https://en.wikipedia.org/wiki/Monkey_Island).  
It's well known that such mixture is quite acid, enough to corrode the
mug's metal where it's poured into.  
This corrosive property, implies that if you have to transport a bit of grog
from one side of the Island to the other,
you have to equip yourself with a bunch of cups and put them in your stash.  
In this way, you can decant the grog when the holding mug is nearly melted.  
So, a *grog cluster* behaves quite similar.  
You can rely on the held content - the *grog* - as far as at least one node
keeps alive in the cluster.

## Table of Contents

* [Description](#description)
* [Goals](#goals)
* [Daemon and CLI](#daemon-and-cli)
* [Usage scenario example](#usage-scenario-example)
* [Network protocol](#network-protocol)
  * [Main concepts](#main-concepts)
  * [Initial phase](#initial-phase)
  * [Live phase](#live-phase)
    * [Incremental message rules](#incremental-message-rules)
  * [Recovery phase](#recovery-phase)
    * [Recovery's functioning](#recoverys-functioning)
    * [Recovery scenario 1](#recovery-scenario-1)
    * [Recovery scenario 2](#recovery-scenario-2)

## Description

As said, grog aims to implement a distributed map kept alive by an arbitrary number
of nodes over a local network.  
Each node holds exactly the same data of any other node.  
This means that the map is not partitioned in any way across the peers.
The only purpose of grog is to provide a shared layer, where an actor can
both read and write data without any restriction.  
Data kept by grog nodes is not persisted in any way.  
This means that, at a certain epoch, a grog's state exists solely when at least
one node is living.  
When all nodes disappear, the state held until that moment is lost forever.

## Goals

* Possibly useful when **developing** a distributed application.
* Immediate usage.
* Zero network configuration.
* No data definition.
* Arbitrary complex type for values.
* json as unique value's format.
* Small operation set to access and manipulate the map:
  * `get`, to get the value of a key from a map
  * `set`, to add or update the value of a key in a map
  * `del`, to remove a key, value pair from a map
* Integrable in programs where an implementation for that language exists.
* **Not** designed to be used in production.
* **Not** meant to be efficient in space.

## Daemon and CLI

grog daemon and CLI is a stand alone program that can be directly executed
allowing to query and modify the map's state.

```text
SYNOPSIS
        grog [-d] [-j <multicast address>] [-p <listening port>] [-l <logging type>] [-v <logging verbosity>] [get <key>] [set <key=value>] [del <key>]

OPTIONS
        -d, --daemon  Spawn a new daemon
        -j, --join    Join the cluster at specified multicast group
        -p, --port    Listen on the specified port
        -l, --log     Specify logging type [console (default), file name]
        -v, --verbosity
                      Specify logging verbosity [off, trace, info (default), warn, err]
        -n, --namespace
                      Specify a namespace when getting/setting a key

        get         Get a key's value
        set         Set a key with its value
        del         Delete a key
```

## Usage scenario example

The following scenario describes an example of interaction between grog daemon
and a custom application that uses grog.

* From `node-1` you spawn a new grog daemon and contextually `set` a key with a value:

```shell
node-1$ grog -d set John='{"name":"John", "surname":"Smith", "age":30}'
daemonize grog ...
updated key=John in default namespace
```

* From `node-2` you `get` the value for the given key:

```shell
node-2$ grog get John
{
    "name":"John", 
    "surname":"Smith", 
    "age":30
}
```

* Then, you write an application:

```Javascript
/**
  Pseudo code of an application that uses an implementation of grog for a programming language
*/

import wxgb.grog

{
  //we get a default constructed grog map; in this form it should be able to interact with all other daemons spawned with no specialized arguments.
  var grogMap = grog.map()
  
  //let's suppose this language supports optional chaining and nil-coaleshing operator.
  let age = node.get("John")?.getAsInt("age") ?? -1
  
  //this should print 30
  print("age:\(age)")

  //this should set a value in the default namespace
  grogMap["Rick"] = "{'name':'Rick', 'surname':'Greene', 'age':57, 'car':'Ford Mustang'}"
}

```

## Network protocol

grog's network protocol uses both multicast UDP and TCP.  
The protocol relies on *timestamps* produced by
grog nodes to serialize the actions over the maps.  
Due to this, hosts's clocks are *strongly* required to be synched.  
The protocol does not contemplate a *master* election between nodes.  
Does not exist a node representing a *source of truth*.  
The unique *qualitative* property of a node is represented
by its *node id* - `nid`.  
The `nid`'s value is set when the node starts and it contains a timestamp
with a nanosecond resolution.  
It is established by the protocol that a node with a *lower* `nid` prevails
on a [timestamp collision](#Incremental-message-rules),
or when a [recovery phase](#recovery-phase) occurs across the cluster.

### Main concepts

When a **node B** starts, it broadcasts an alive message over the
UDP multicast channel.  
Supposing there is already another **node A** running, it will broadcast
back its alive message.  
**Node B** thus, uses the **node A** alive message in order to ask
for a map's *snapshot*.  
A snapshot is requested by a node to another node via TCP.  
After the snapshot has been successfully transmitted, socket closes.  
Every map's state variation will be disseminated via *incremental*
packets on UDP.  

So, there are two main flows:

* **snapshot**, solely via TCP
* **incremental**, solely via UDP multicast

### Initial phase

When a node starts, it performs the following steps:

* Send an alive `A` message over the UDP multicast channel

```javascript
{
  type: "A",                   //message type, A (alive)
  ts: 0,                       //the map's timestamp at the sending epoch of the message, zero when this is the first A message produced
  nid: ${node-starting-TS},    //the node-id, it is set at the starting epoch of the node
  seqno: 0,                    //the last sequence number produced by this node, it is incremented every time an I message is produced
  address: "${ip}:${port}"     //the node's ip:port where it listen for snapshot requests
}
```

* If an `A` message is received from an another node, then connect to obtain a snapshot `S` message.
* If no `A` messages are received in useful time, then set a map's timestamp and go on with the [live phase](#live-phase).

A node serves a snapshot when a TCP connection is established at the address specified in the `A` message.  
The json entity containing a snapshot is streamed over the TCP channel.  
The first 4 bytes received on the stream denote the length (in bytes) of the subsequent json entity.

```javascript
${message-length}                 //4 bytes (big endian), containing the subsequent json message length
{
  type: "S",                      //message type, S (snapshot)
  ts: ${map-TS},                  //the map's timestamp at the sending epoch of the message
  nid: ${node-starting-TS},       //the node-id, it is set at the starting epoch of the node
  seqnos: [{nid: ${nid-0},
            seqno :${seqno-0}},
           {nid: ${nid-1},
            seqno :${seqno-1}},
           {nid: ${nid-2},
            seqno :${seqno-2}},
           ...
           ],                     //an array containing all the sequence numbers of every node contributed updating the map
  snapshot: {...}                 //the map's snapshot data, see below for format's details
}
```

The `snapshot` field in the `S` message has the following format:

```javascript
{
  ts: ${map-TS},                      //the map's timestamp, it's the last event timestamp

  snapshot-ns: [                      //an array of namespaced items
    {
      ns: ${namespace-0},             //the item namespace

      seqnos: [                       //an array of pairs K:V, each one falling into the same parent's namespace
        {
          ts: ${event-TS-0},          //each K:V pair has associated an event's TS
          nid: ${nid-0},              //each K:V pair has associated the node id that produced the event
          key: ${K-0},
          val: ${V-0}
        },
        {
          ts: ${event-TS-1},
          nid: ${nid-1},
          key: ${K-1},
          val: ${V-1}
        },

        ...
      ]
    },

  ...
  ]
}
```

* A node receiving a snapshot is required to:
  * Collect all the receiving, incremental `I` messages sent by other
nodes during this phase.
  * After have received the snapshot, process all the collected `I`
messages with the [Incremental message rules](#Incremental-message-rules).

If a node receiving a snapshot experiences a disconnection from the serving
node before
the entire stream has been transferred, then it is required to retry the
entire procedure.  
If no node is available to serve a snapshot, then set a map's timestamp and
go on with the [live phase](#live-phase).

### Live phase

After having successfully completed the [initial phase](#initial-phase), a node can transit into
the live phase.  
This is the regular final state of a running node.  
When in live phase, a node must:

* process all the incoming `I` messages.
* respond to all the incoming TCP connections and serve the snapshots.
* send unsolicited `A` messages with a frequency of 20 seconds.
* send an `A` message in reply to received `A` messages with `ts = 0` (new nodes).

When the node is part of an application, it processes the changes requested
by the applicative layer.  
In this case, the node also actively produces `I` messages.

An `I` message has the following structure:

```javascript
{
  type: "I",                      //message type, I (incremental)
  ts: ${I-TS},                    //the I timestamp, it denotes the timestamp of this event
  nid: ${node-starting-TS},       //the node-id, it is set at the starting epoch of the node
  seqno: ${last-seqno},           //the last sequence number produced by this node
  op: ${operation-code},          //the operation associated with the message: set or del
  ns: ${namespace},               //item's namespace
  key: ${K},                      //item's key
  val: ${V}                       //item's value
}
```

When applying incremental messages a node is required to adhere the
following rules:

#### Incremental message rules

* Check if the received `I` message is in-sequence in respect of its own `nid` flow.
  * Discard all received messages with a `seqno` lesser than the expected.
  * Start the [recovery phase](#recovery-phase) when receiving a message with a `seqno` greater than the expected.
* When the received message is in-sequence, lookup in the map for an entry with the given key:`K`:
  * If an entry doesn't exists, then apply the incoming operation.
  * If an entry exists, evaluate the entry's `ts`:
    * If the message's `ts` is lesser than entry's `ts`, then discard the incoming operation.
    * If the message's `ts` is greater than entry's `ts`, then apply the incoming operation.
    * If the message's `ts` is equal to the entry's `ts` (*timestamp collision*), then evaluate the entry's `nid`:
      * Apply the incoming operation only if the message's `nid` is lesser than the entry's `nid`.

### Recovery phase

This is a special state that occurs when certain erroneous
conditions are detected on a node.  
A recovery phase can resolve inside a node or, at worst,
can potentially affect the state of the entire cluster.  
Recall that every node can *independently* produce data and
there is no *source of truth*.  
With these premises, say if it is more correct to save a state
rather than another, is a total arbitrary matter.  
Because a choice must be done, when a recovery state initiates,
it was chosen to make prevail the state of an older node in the cluster.

Erroneous conditions can occur due to UDP unreliability.  
Indeed, a node can:

* Receive packets out of sequence.
* Lose packets.

A node detects a *gap* when it receives an OOS packet; this is possible with:

* `A` with a `seqno` higher than the expected.
* `I`  with a `seqno` higher than the expected.

When a gap is detected with an `I` message, a node is allowed to wait
a grace period of 100 ms trying to receive the missing packet(s).  
If the gap resolves spontaneously, thus the missing packed are received and reordered,
no further actions are required.  
If instead, a gap is detected with an `A` message or the grace period
elapses, the node must initiate a recovery phase.

#### Recovery's functioning

As said, the protocol chooses to prevail the state of an older node.  
So, a node starting a recovery phase, evaluates its `nid` against the
`nid` of the received out-of-sequence packet.  

When:

* The node's `nid` is greater than the counterpart's `nid`:
  * Trigger a snapshot request, to be done after 2 seconds, against the counterpart.
  * If, during the countdown, an OOS `A` message with an even lesser `nid` is received,
    then reset the countdown and *push-front* a new snapshot request with the new counterpart.
  * Requests must be try in order, from the smallest `nid` to the greatest; the process stops
    as soon as a request completes successfully.
* The node's `nid` is lesser than the counterpart's `nid`
  * Broadcast an artificial OOS `A` message and update the internal `seqno` accordingly.

#### Recovery scenario 1

There are 4 nodes forming a cluster: N1, N2, N3 and N4.  
N1 is the oldest, N2 has spawned after N1 and so on.  
At a certain epoch, N3 produces an `I` message that is not received by N4.  
When N3 produces an `A` message, N4 detects a gap because the `seqno` in the message is
not the expected one.

```text
  [1]               [2]
N1 <---           N1 <--- [ok]
      |                 |
N2 <---           N2 <--- [ok]
      |                 |
N3 ---> (I)       N3 ---> (A)
      |                 |
N4  X--           N4 <--- [gap detected]

```

Because N4's `nid` is greater than N3's `nid`,
N4 triggers a request for a snapshot to N3 in 2 seconds.  

```text
  [3]               [4]
N1                N1

N2                N2

N3 <--<           N3 >--> (S)
      | [TCP]           |
N4 >-->           N4 <--<

```

N4 is now recovered.

#### Recovery scenario 2

At a certain epoch, N3 produces an `I` message that is not received by N2.  
When N3 produces an `A` message, N2 detects a gap because the `seqno` in the message is
not the expected one.

```text
  [1]               [2]
N1 <---           N1 <--- [ok]
      |                 |
N2  X--           N2 <--- [gap detected]
      |                 |
N3 ---> (I)       N3 ---> (A)
      |                 |
N4 <---           N4 <--- [ok]

```

Because N2's `nid` is lesser than N3's `nid`, N2 broadcast an artificial OOS `A` message.  

```text
  [3]
N1 <--- [gap detected]
      |
N2 ---> (OOS A)
      |
N3 <--- [gap detected]
      |
N4 <--- [gap detected]

```

When N2 produces an OOS `A` message, all the other nodes detect a gap.  
Because N1's `nid` is lesser than N2's `nid`, N1 broadcast an artificial OOS `A` message.  
Because N3's `nid` is greater than N2's `nid`,
N3 triggers a request for a snapshot to N2 in 2 seconds.  
Because N4's `nid` is greater than N2's `nid`,
N4 triggers a request for a snapshot to N2 in 2 seconds.  

```text
  [4]
N1 ---> (OOS A)
      |
N2 <--- [gap detected]
      |
N3 <--- [gap detected] [pending snapshot request to N2]
      |
N4 <--- [gap detected] [pending snapshot request to N2]

```

When N1 produces an OOS `A` message, all the other nodes detect a gap.  
Because N2's `nid` is greater than N1's `nid`,
N2 triggers a request for a snapshot to N1 in 2 seconds.  
Because N3's `nid` is greater than N1's `nid`,
N3 triggers a request for a snapshot to N1 in 2 seconds.  
Because N4's `nid` is greater than N1's `nid`,
N4 triggers a request for a snapshot to N1 in 2 seconds.

```text
  [5]                [6]
N1 <-------        N1 >-------
      | | |              | | |
N2 >--> | |        N2 <--< | |
        | | [TCP]          | | (S)x3
N3 >----> |        N3 <----< |
          |                  |
N4 >------>        N4 <------<

```

N2 is now recovered.
