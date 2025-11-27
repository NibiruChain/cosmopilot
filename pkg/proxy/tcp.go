package proxy

import (
	"fmt"
	"io"
	"net"
	"sync"

	log "github.com/sirupsen/logrus"
)

type TCP struct {
	laddr, raddr *net.TCPAddr
	listener     *net.TCPListener
	runOnce      bool
}

func NewTCPProxy(localAddr, remoteAddr string, failOnClose bool) (*TCP, error) {
	laddr, err := net.ResolveTCPAddr("tcp", localAddr)
	if err != nil {
		return nil, err
	}
	raddr, err := net.ResolveTCPAddr("tcp", remoteAddr)
	if err != nil {
		return nil, err
	}

	return &TCP{
		laddr:   laddr,
		raddr:   raddr,
		runOnce: failOnClose,
	}, nil
}

func (p *TCP) Start() error {
	var err error
	log.Infof("starting tcp proxy at %v", p.laddr)
	p.listener, err = net.ListenTCP("tcp", p.laddr)
	if err != nil {
		return err
	}
	defer p.listener.Close()

	for {
		lconn, err := p.listener.AcceptTCP()
		if err != nil {
			log.Errorf("failed to accept connection: %v", err)
			continue
		}

		log.WithFields(log.Fields{
			"laddr": p.laddr,
			"raddr": p.raddr,
		}).Info("handling TCP connection")

		if err = p.handle(lconn); err != nil {
			log.Errorf("failed to handle connection: %v", err)
			continue
		}
		if p.runOnce {
			return err
		}
	}
}

func (p *TCP) Stop() error {
	return p.listener.Close()
}

func (p *TCP) handle(lconn *net.TCPConn) error {
	rconn, err := net.DialTCP("tcp", nil, p.raddr)
	if err != nil {
		lconn.Close()
		return fmt.Errorf("failed to dial upstream: %v", err)
	}

	// Use sync.Once to ensure connections are closed exactly once
	var lconnClose, rconnClose sync.Once
	closeLconn := func() { lconnClose.Do(func() { lconn.Close() }) }
	closeRconn := func() { rconnClose.Do(func() { rconn.Close() }) }

	wg := sync.WaitGroup{}
	wg.Add(2)

	go func() {
		defer wg.Done()
		defer closeLconn()
		defer closeRconn()
		if _, err := io.Copy(rconn, lconn); err != nil {
			log.WithFields(log.Fields{
				"laddr": p.laddr,
				"raddr": p.raddr,
			}).Tracef("error copying from %v: %v", lconn.RemoteAddr(), err)
		}
		log.WithFields(log.Fields{
			"laddr": p.laddr,
			"raddr": p.raddr,
		}).Tracef("finished copying from %v", lconn.RemoteAddr())
	}()

	go func() {
		defer wg.Done()
		defer closeLconn()
		defer closeRconn()
		if _, err := io.Copy(lconn, rconn); err != nil {
			log.WithFields(log.Fields{
				"laddr": p.laddr,
				"raddr": p.raddr,
			}).Tracef("error copying to %v: %v", lconn.RemoteAddr(), err)
		}
		log.WithFields(log.Fields{
			"laddr": p.laddr,
			"raddr": p.raddr,
		}).Tracef("finished copying to %v", lconn.RemoteAddr())
	}()

	wg.Wait()
	return nil
}
