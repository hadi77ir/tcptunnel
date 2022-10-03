// Copyright 2022 Mohammad Hadi Hosseinpour
// Copyright 2017 Burak Sezer
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"io"
	"net"
	"net/url"
	"os"
	"sync"
	"time"
)

import "golang.org/x/net/proxy"

type client struct {
	listenAddress   string
	targetAddress   string
	proxyAddress    string
	keepAlivePeriod time.Duration
	dialTimeout     time.Duration
	wg              sync.WaitGroup
	errChan         chan error
	signal          chan os.Signal
	done            chan struct{}
}

func newClient(listenAddress string, targetAddress string, proxyAddress string, dialTimeout time.Duration,
	keepAlivePeriod time.Duration, sigChan chan os.Signal) *client {

	return &client{
		listenAddress:   listenAddress,
		targetAddress:   targetAddress,
		proxyAddress:    proxyAddress,
		keepAlivePeriod: dialTimeout * time.Second,
		dialTimeout:     keepAlivePeriod * time.Second,
		wg:              sync.WaitGroup{},
		errChan:         make(chan error, 1),
		signal:          sigChan,
		done:            make(chan struct{}),
	}
}

func (c *client) connCopy(dst, src net.Conn, copyDone chan struct{}) {
	defer c.wg.Done()
	defer func() {
		copyDone <- struct{}{}
	}()
	_, err := io.Copy(dst, src)
	if err != nil {
		opErr, ok := err.(*net.OpError)
		switch {
		case ok && opErr.Op == "readfrom":
			return
		case ok && opErr.Op == "read":
			return
		default:
		}
		log.Errorf("failed to copy connection from %s to %s: %s",
			src.RemoteAddr(), dst.RemoteAddr(), err)
	}
}

func (c *client) Run() error {
	var proxyURL *url.URL
	var err error
	if c.proxyAddress != "" {
		proxyURL, err = url.Parse(c.proxyAddress)
		if err != nil {
			log.Fatalf("could not parse proxy URL: %s", err)
			return err
		}
	}

	listener, err := net.Listen("tcp", c.listenAddress)
	if err != nil {
		log.Fatalf("could not start listening: %s", err)
		return err
	}
	log.Infof("Listening port opened on %s", c.listenAddress)

	var dialer proxy.Dialer
	// default should be direct
	if dialer == nil {
		dialer = proxy.Direct
	}

	// if proxy has been defined, chain direct with proxy (proxy -> direct)
	if proxyURL != nil {
		if dialer, err = proxy.FromURL(proxyURL, dialer); err != nil {
			log.Fatalf("could not construct proxy: %s", err)
			return err
		}
	}

	go c.serve(listener, dialer)

	// wait...
	select {
	// Wait for SIGINT or SIGTERM
	case <-c.signal:
		log.Infof("received shutdown signal from user")
	// Wait for a listener error
	case <-c.done:
	}

	// Signal all running goroutines to stop.
	c.shutdown()

	log.Infof("stopping proxy client: %s", c.listenAddress)
	if err = listener.Close(); err != nil {
		log.Errorf("failed to close listener: %s", err)
	}

	ch := make(chan struct{})
	go func() {
		defer close(ch)
		c.wg.Wait()
	}()

	select {
	case <-ch:
	case <-time.After(time.Duration(10) * time.Second):
		log.Warnf("some goroutines will be stopped forcefully")
	}
	return <-c.errChan

}

func (c *client) serve(listener net.Listener, dialer proxy.Dialer) error {
	// accept loop
	for {
		accepted, err := listener.Accept()
		if err != nil {
			log.Fatalf("error accepting connection: %s", err)
			return err
		}
		log.Infof("accepted connection from %s on %s", accepted.RemoteAddr(), accepted.LocalAddr())

		// when accepted, dial remote
		dialed, err := dialer.Dial("tcp", c.targetAddress)
		if err != nil {
			log.Errorf("error dialing remote target: %s", err)
			accepted.Close()
			continue
		}

		c.wg.Add(1)
		// tunnel the connection
		go c.handleConn(accepted, dialed)
	}
}

func (c *client) handleConn(accepted net.Conn, remote net.Conn) {
	defer c.wg.Done()
	defer accepted.Close()
	defer remote.Close()

	log.Infof("tunneling connection from %s to %s", accepted.RemoteAddr(), remote.RemoteAddr())

	ch := make(chan struct{})
	c.wg.Add(1)
	go c.duplexCopy(accepted, remote, ch)
	select {
	case <-c.done:
	case <-ch:
	}
}

func (c *client) shutdown() {
	select {
	case <-c.done:
		return
	default:
	}
	close(c.done)
}

func (c *client) duplexCopy(conn, rConn net.Conn, ch chan struct{}) {
	defer c.wg.Done()

	// close ch, clientConn waits until it will be closed.
	defer close(ch)
	copyDone := make(chan struct{}, 2)

	c.wg.Add(2)
	go c.connCopy(rConn, conn, copyDone)
	go c.connCopy(conn, rConn, copyDone)
	// rConn and conn will be closed by defer calls in clientConn. There is nothing to do here.
	<-copyDone
}
