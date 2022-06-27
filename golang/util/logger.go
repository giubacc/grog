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
	"io"
	"log"
	"os"
)

type LogLevel int

const (
	Trc = iota
	Inf
	Wrn
	Err
	Cri
	Off
)

type LogLevelStr string

const (
	TraceStr    = "trc"
	InfoStr     = "inf"
	WarnStr     = "wrn"
	ErrStr      = "err"
	CriticalStr = "cri"
	OffStr      = "off"
)

var LogLevelStr2LvL = map[string]LogLevel{TraceStr: Trc, InfoStr: Inf, WarnStr: Wrn, ErrStr: Err, CriticalStr: Cri, OffStr: Off}

type Logger struct {
	Lgr    *log.Logger
	LgrLvl LogLevel
	Class  string
	Out    io.Writer

	//opt logger fd
	l_fd *os.File
}

func (lgr *Logger) Init(class string, cfg *Config) error {
	lgr.LgrLvl = LogLevelStr2LvL[cfg.LogLevel]
	lgr.Class = class
	if cfg.LogType == "shell" {
		lgr.Out = os.Stdout
	} else {
		var err error
		if lgr.l_fd, err = os.OpenFile(class+cfg.LogType, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0644); err != nil {
			fmt.Println(err.Error())
			return &GrogError{RetCode_IOERR}
		}
		lgr.Out = lgr.l_fd
	}

	lgr.Lgr = log.New(lgr.Out, "", log.Lmsgprefix|log.Ltime|log.Lmicroseconds)
	return nil
}

func (lgr *Logger) Stop() {
	if lgr.l_fd != nil {
		lgr.l_fd.Close()
	}
}

func (lgr *Logger) Trc(format string, v ...interface{}) {
	if lgr.LgrLvl > Trc {
		return
	}
	lgr.Lgr.SetPrefix("[" + lgr.Class + TraceStr + "] ")
	lgr.Lgr.Printf(format, v...)
}

func (lgr *Logger) Inf(format string, v ...interface{}) {
	if lgr.LgrLvl > Inf {
		return
	}
	lgr.Lgr.SetPrefix("[" + lgr.Class + InfoStr + "] ")
	lgr.Lgr.Printf(format, v...)
}

func (lgr *Logger) Wrn(format string, v ...interface{}) {
	if lgr.LgrLvl > Wrn {
		return
	}
	lgr.Lgr.SetPrefix("[" + lgr.Class + WarnStr + "] ")
	lgr.Lgr.Printf(format, v...)
}

func (lgr *Logger) Err(format string, v ...interface{}) {
	if lgr.LgrLvl > Err {
		return
	}
	lgr.Lgr.SetPrefix("[" + lgr.Class + ErrStr + "] ")
	lgr.Lgr.Printf(format, v...)
}

func (lgr *Logger) Cri(format string, v ...interface{}) {
	if lgr.LgrLvl > Cri {
		return
	}
	lgr.Lgr.SetPrefix("[" + lgr.Class + CriticalStr + "] ")
	lgr.Lgr.Printf(format, v...)
}

var defLog Logger

func DefLog() *Logger {
	if defLog.Lgr == nil {
		if err := defLog.Init("dflt.", &Config{LogType: "shell", LogLevel: "trc"}); err != nil {
			return nil
		}
	}
	return &defLog
}
