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
	"encoding/binary"
	"grog/util"
	"net"
	"time"

	"github.com/sirupsen/logrus"
)

type Streamer struct {
	//config
	Cfg *util.Config

	//logger
	logger *logrus.Logger
}

func (s *Streamer) Init() error {
	//logger init
	s.logger = util.GetLogger("strm", s.Cfg)

	return nil
}

func (s *Streamer) SendSnapshotBuffer(conn net.Conn, buff []byte) error {
	if s.Cfg.VerbLevel >= util.VL_TRACE {
		s.logger.WithFields(logrus.Fields{
			"type": "S",
			"msg":  string(buff),
		}).Trace(">>")
	}

	//put buffer's length in front of the outgoing stream
	outgBuff := make([]byte, len(buff)+4)
	binary.LittleEndian.PutUint32(outgBuff[0:4], uint32(len(buff)))
	copy(outgBuff[4:], buff)

	nsent, err := conn.Write(outgBuff)
	if err != nil {
		s.logger.Errorf("sending data msg:%s", err.Error())
		return err
	}
	s.logger.Tracef("[TCP] sent %d bytes to:%s", nsent, conn.RemoteAddr().String())
	return nil
}

func (s *Streamer) DialAndReceiveSnapshotBuffer(address string) ([]byte, error) {
	var d net.Dialer
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	conn, err := d.DialContext(ctx, "tcp", address)
	if err != nil {
		s.logger.Errorf("dialing to:%s", address)
		return nil, err
	}
	defer conn.Close()

	header := make([]byte, 4)
	for {
		nread, err := conn.Read(header)
		if err != nil {
			s.logger.Errorf("receiving snapshot header:%s", err.Error())
			return nil, err
		}
		if nread >= 4 {
			bodyLen := binary.LittleEndian.Uint32(header[0:])
			bodyBuffer := make([]byte, bodyLen)

			if nread, err := conn.Read(bodyBuffer); err != nil {
				s.logger.Errorf("receiving snapshot body:%s", err.Error())
				return nil, err
			} else {
				s.logger.Tracef("[TCP] received %d (body) bytes from:%s", nread, conn.RemoteAddr().String())
				if s.Cfg.VerbLevel >= util.VL_TRACE {
					s.logger.WithFields(logrus.Fields{
						"type": "S",
						"msg":  string(bodyBuffer),
					}).Trace("<<")
				}
				return bodyBuffer, nil
			}
		}
	}
}
