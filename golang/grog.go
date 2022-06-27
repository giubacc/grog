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

package main

import (
	"flag"
	"grog/node"
	"grog/util"
	"strings"
)

var nd node.Node

func ParseGetArg(getArg string) error {
	nd.Cfg.Op = util.OP_GET
	nd.Cfg.Key = getArg
	return nil
}

func ParseSetArg(setArg string) error {
	var tokens = strings.Split(setArg, "=")
	if len(tokens) != 2 {
		return &util.GrogError{Code: util.RetCode_BADCFG}
	}
	nd.Cfg.Op = util.OP_SET
	nd.Cfg.Key = tokens[0]
	nd.Cfg.Val = tokens[1]
	return nil
}

func ParseDelArg(delArg string) error {
	nd.Cfg.Key = delArg
	nd.Cfg.Op = util.OP_DEL
	return nil
}

func main() {
	flag.BoolVar(&nd.Cfg.Daemonize, "d", false, "Spawn a new daemon")
	flag.StringVar(&nd.Cfg.MulticastAddress, "j", "232.232.200.82", "Join the cluster at specified multicast group")
	flag.UintVar(&nd.Cfg.MulticastPort, "jp", 8745, "Join the cluster at specified multicast group port")
	flag.UintVar(&nd.Cfg.ListeningPort, "p", 31582, "Listen on the specified port")
	flag.IntVar(&nd.Cfg.NodeSynchDuration, "syd", 1, "Node synchronization phase duration in seconds")
	flag.IntVar(&nd.Cfg.LoopReactivity, "lr", 100, "Loop reactivity in milliseconds")

	flag.StringVar(&nd.Cfg.LogType, "l", "shell", "Specify logging type [shell (default), file name]")
	flag.StringVar(&nd.Cfg.LogLevel, "v", "inf", "Specify logging verbosity [off, trc, inf (default), wrn, err]")

	flag.StringVar(&nd.Cfg.Namespace, "n", "", "Specify a namespace when getting/setting a key")
	flag.Func("get", "Get a key's value", ParseGetArg)
	flag.Func("set", "Set a key with its value", ParseSetArg)
	flag.Func("del", "Delete a key", ParseDelArg)

	flag.StringVar(&nd.Cfg.NetInterfaceName, "inet", "eth0", "Specify a network interface's name")
	flag.UintVar(&nd.Cfg.VerbLevel, "vl", 5, "Verbosity level")

	flag.Parse()
	nd.Run()
}
