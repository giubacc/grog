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
)

type RetCode int

const (
	RetCode_UNKERR = -1000 /**< unknown error */

	//system errors
	RetCode_SCKERR = -105 /**< socket error */
	RetCode_DBERR  = -104 /**< database error */
	RetCode_IOERR  = -103 /**< I/O operation fail */
	RetCode_MEMERR = -102 /**< memory error */
	RetCode_SYSERR = -101 /**< system error */

	//generic error
	RetCode_UNVRSC = -2 /**< unavailable resource */
	RetCode_GENERR = -1 /**< generic error */

	//success, failure [0,1]
	RetCode_OK    = 0 /**< operation ok */
	RetCode_KO    = 1 /**< operation fail */
	RetCode_EXIT  = 2 /**< exit required */
	RetCode_RETRY = 3 /**< request retry */
	RetCode_ABORT = 4 /**< operation aborted */

	//generics
	RetCode_UNSP     = 100 /**< unsupported */
	RetCode_NODATA   = 101 /**< no data */
	RetCode_NOTFOUND = 102 /**< not found */
	RetCode_TIMEOUT  = 103 /**< timeout */

	//proc. specific
	RetCode_BADARG  = 300 /**< bad argument */
	RetCode_BADIDX  = 301 /**< bad index */
	RetCode_BADSTTS = 302 /**< bad status */
	RetCode_BADCFG  = 303 /**< bad configuration */

	//network specific
	RetCode_DRPPKT  = 400 /**< packet dropped*/
	RetCode_MALFORM = 401 /**< packet malformed */
	RetCode_SCKCLO  = 402 /**< socket closed */
	RetCode_SCKWBLK = 403 /**< socket would block */
	RetCode_PARTPKT = 404 /**< partial packet */
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
