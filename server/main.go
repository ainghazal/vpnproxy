package main

import (
	"net"
	"os"
	"strings"

	"github.com/jessevdk/go-flags"
	log "github.com/sirupsen/logrus"
)

var opts struct {
	Source string `long:"source" default:":3000" description:"Source port to listen on"`
	Target string `long:"target" description:"Target address to forward to"`
	Quiet  bool   `long:"quiet" description:"whether to print logging info or not"`
	Buffer int    `long:"buffer" default:"1024" description:"max buffer size for the socket io"`
}

func main() {
	_, err := flags.Parse(&opts)
	if err != nil {
		if !strings.Contains(err.Error(), "Usage") {
			log.Printf("error: %v\n", err.Error())
			os.Exit(1)
		} else {
			os.Exit(0)
		}
	}

	if opts.Quiet {
		log.SetLevel(log.WarnLevel)
	}

	targetAddr, err := net.ResolveUDPAddr("udp", opts.Target)
	if err != nil {
		log.WithError(err).Fatal("Could not resolve target address:", opts.Target)
		return
	}

	_, err = net.ResolveUDPAddr("udp", opts.Source)
	if err != nil {
		log.WithError(err).Fatal("Could not resolve source address:", opts.Source)
		return
	}

	log.Printf(">> Starting udpproxy server, Source at %v/tcp, Target at %v/udp...\n", opts.Source, opts.Target)

	listener, err := net.Listen("tcp", opts.Source)
	if err != nil {
		log.WithError(err).Fatal("Could not listen on address:", opts.Source)
		return
	}
	defer listener.Close()

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.WithError(err).Fatal("Could not listen on local address:", opts.Source)
			return
		}

		remoteConn, err := net.DialUDP("udp", nil, targetAddr)
		if err != nil {
			log.WithError(err).Fatal("Could not connect to target address:", opts.Target)
			continue
		}
		doProxy(conn, remoteConn)
	}
}

func doProxy(local net.Conn, remote *net.UDPConn) {
	// we defer copy remote (udp) -> local (tcp)
	go func() {
		for {
			b := make([]byte, opts.Buffer)
			n, addr, err := remote.ReadFrom(b[:])
			if err != nil {
				log.WithError(err).Error("Could not receive a packet")
				return
			}

			log.WithField("addr", addr.String()).WithField("bytes", n).WithField("payload", string(b[:n])).Info("Packet received")
			if _, err := local.Write(b[0:n]); err != nil {
				log.WithError(err).Warn("Could not forward packet.")
			}
		}
	}()

	// now we loop-copy local (tcp) -> remote (udp)
	for {
		// Read UDP socket from remote
		b := make([]byte, opts.Buffer)
		n, err := local.Read(b[:])
		if err != nil {
			log.WithError(err).Error("Could not read from local TCP connection")
		}
		// Write the packet's contents to the remote UDP conn.
		n, err = remote.Write(b[:n])
		if err != nil {
			log.WithError(err).Error("Could not write")
		}
		/*
			if err != nil {
				doneChan <- err
				return
			}
		*/

		log.Printf("packet-written: bytes=%d to=%s\n", n, opts.Target)
	}
}
