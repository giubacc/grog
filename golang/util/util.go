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

import (
	"fmt"
	"os"

	"github.com/sirupsen/logrus"
)

type RetCode int

const (
	RetCode_OK = iota
	RetCode_KO
	RetCode_NOP
	RetCode_EXIT
	RetCode_BADCFG
)

type GrogError struct {
	Code RetCode
}

func (e *GrogError) Error() string {
	return fmt.Sprintf("GrogError:%d", e.Code)
}

type Config struct {
	Daemonize         bool
	NetInterfaceName  string
	MulticastAddress  string
	MulticastPort     uint
	ListeningPort     uint
	NodeSynchDuration int //seconds
	LoopReactivity    int //milliseconds

	//CLI
	Namespace string
	Key       string
	Val       string
	Op        uint

	//logging
	LogType   string
	LogLevel  string
	VerbLevel uint
}

const (
	VL_LOW   = 5
	VL_HIGH  = 10
	VL_TRACE = 20
)

//status codes
const (
	ZERO = iota
	INIT
	SYNC
	LOOP
)

//request types
const (
	STATUS = iota
	GIVE_TS
)

type Request struct {
	ReqCode  uint
	Response chan interface{}
}

type LogLevelStr string

const (
	TraceStr = "trc"
	InfoStr  = "inf"
	WarnStr  = "wrn"
	ErrStr   = "err"
	FatalStr = "fat"
)

var LogLevelStr2LvL = map[string]logrus.Level{
	TraceStr: logrus.TraceLevel,
	InfoStr:  logrus.InfoLevel,
	WarnStr:  logrus.WarnLevel,
	ErrStr:   logrus.ErrorLevel,
	FatalStr: logrus.FatalLevel}

func GetLogger(class string, cfg *Config) *logrus.Logger {
	logger := &logrus.Logger{
		Formatter: new(logrus.TextFormatter),
		Hooks:     make(logrus.LevelHooks),
		Level:     LogLevelStr2LvL[cfg.LogLevel],
	}

	if cfg.LogType == "shell" {
		logger.Out = os.Stdout
	} else {
		if l_fd, err := os.OpenFile(class+cfg.LogType, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0644); err != nil {
			fmt.Println(err.Error())
			return nil
		} else {
			logger.Out = l_fd
		}
	}
	return logger
}
