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
	log.Infof("starting tcp proxy at %v", p.laddr)
	listener, err := net.ListenTCP("tcp", p.laddr)
	if err != nil {
		return err
	}
	defer listener.Close()

	for {
		lconn, err := listener.AcceptTCP()
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

func (p *TCP) handle(lconn *net.TCPConn) error {
	rconn, err := net.DialTCP("tcp", nil, p.raddr)
	if err != nil {
		lconn.Close()
		return fmt.Errorf("failed to dial upstream: %v", err)
	}

	wg := sync.WaitGroup{}
	wg.Add(2)

	go func() {
		defer lconn.Close()
		defer rconn.Close()
		io.Copy(rconn, lconn)
		log.WithFields(log.Fields{
			"laddr": p.laddr,
			"raddr": p.raddr,
		}).Tracef("finished copying from %v", lconn.RemoteAddr())
		wg.Done()
	}()

	go func() {
		defer lconn.Close()
		defer rconn.Close()
		io.Copy(lconn, rconn)
		log.WithFields(log.Fields{
			"laddr": p.laddr,
			"raddr": p.raddr,
		}).Tracef("finished copying to %v", lconn.RemoteAddr())
		wg.Done()
	}()

	wg.Wait()
	return nil
}
