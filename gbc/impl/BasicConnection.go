// MIT License
//
// Copyright (conn) 2018 dualface
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package impl

import (
    "io"
    "net"

    "github.com/dualface/go-cli-colorlog"
    "github.com/dualface/go-gbc/gbc"
)

type (
    BasicConnection struct {
        conn    net.Conn
        conf    *gbc.ConnectionConfig
        running bool
        mc      chan gbc.RawMessage
        parser  *RawMessageParser
    }
)

func NewBasicConnection(rawConn net.Conn, conf *gbc.ConnectionConfig) gbc.Connection {
    if conf == nil {
        conf = gbc.DefaultConnectionConfig
    }

    c := &BasicConnection{
        conn:    rawConn,
        conf:    conf,
        running: false,
        parser:  NewRawMessageParser(),
    }
    return c
}

// interface Connection

func (c *BasicConnection) SetMessageChan(mc chan gbc.RawMessage) {
    c.mc = mc
}

func (c *BasicConnection) Start() {
    if !c.running {
        c.running = true
        go c.loop()
    }
}

func (c *BasicConnection) Close() error {
    return c.conn.Close()
}

func (c *BasicConnection) Write(b []byte) (int, error) {
    return c.conn.Write(b)
}

// private

func (c *BasicConnection) loop() {
    failure := 0
    // use double buffer
    halfBufSize := c.conf.ReadBufferSize
    buf := make([]byte, halfBufSize*2, halfBufSize*2)
    offset := 0

    for {
        if failure >= c.conf.ReadFailureLimit {
            // stop read
            break
        }

        avail, err := c.conn.Read(buf[offset : offset+halfBufSize])
        if err != nil {
            if err != io.EOF {
                clog.PrintWarn("reading failed on %s, %s", c.conn.RemoteAddr(), err)
                failure++
                continue // try again
            } else if avail == 0 {
                break // conn closed
            }
        } else {
            // reset read failure counter
            failure = 0
        }

        for avail > 0 {
            writeLen, err := c.parser.WriteBytes(buf[offset : offset+avail])
            avail -= writeLen
            offset += writeLen
            if err != nil {
                clog.PrintWarn("parsing bytes failed, %s", err)
            }

            msg := c.parser.FetchMessage()
            if msg != nil {
                if c.mc != nil {
                    c.mc <- msg
                } else {
                    clog.PrintWarn("connection discard message")
                }
            }
        }

        if offset >= halfBufSize {
            offset = 0
        }
    }
}