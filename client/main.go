package main

import (
	"encoding/binary"
	"net"
	"os"
	"strings"

	"github.com/jessevdk/go-flags"
	log "github.com/sirupsen/logrus"
)

var (
	localAddr net.Addr
)

var opts struct {
	Source string `long:"source" default:":2203" description:"Source port to listen on"`
	Target string `long:"target" description:"Target address to forward to"`
	Quiet  bool   `long:"quiet" description:"whether to print logging info or not"`
	Buffer int    `long:"buffer" default:"10240" description:"max buffer size for the socket io"`
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

	targetAddr, err := net.ResolveTCPAddr("tcp", opts.Target)
	if err != nil {
		log.WithError(err).Fatal("Could not resolve target address:", opts.Target)
		return
	}

	log.Printf(">> Starting udpproxy client, Source at %v/udp, Target at %v/tcp...\n", opts.Source, opts.Target)

	pc, err := net.ListenPacket("udp", opts.Source)
	if err != nil {
		log.WithError(err).Fatal("Could not listen on address:", opts.Source)
		return
	}
	defer pc.Close()

	for {
		// TODO keep connection map
		remoteConn, err := net.DialTCP("tcp", nil, targetAddr)
		if err != nil {
			log.WithError(err).Fatal("Could not connect to target address:", opts.Target)
			continue
		}
		doProxy(remoteConn, pc)
	}
}

func doProxy(remote net.Conn, local net.PacketConn) {
	// copy remote (tcp) -> local (udp)
	go func() {
		bufLen := make([]byte, 2)
		for {
			// Simple framing:
			// Read 2 bytes from TCP stream, update bufLenlen
			n, err := remote.Read(bufLen)
			if err != nil {
				log.WithError(err).Error("Could not read len from local TCP connection")
			}
			l := binary.BigEndian.Uint16(bufLen)

			// Read l bytes from remote TCP socket
			b := make([]byte, l)
			n, err = remote.Read(b[:])
			// Write the payload to the local UDP port
			n, err = local.WriteTo(b[:], localAddr)
			if err != nil {
				log.Println("error writing")
			}

			if n != 0 {
				log.Printf("packet-written: bytes=%d to=%s\n", n, localAddr.String())
			}
		}
	}()

	// copy local (udp) -> remote (tcp)
	for {
		b := make([]byte, opts.Buffer)
		n, addr, err := local.ReadFrom(b)

		if err != nil {
			log.WithError(err).Error("Could not receive a packet")
			return
		}
		localAddr = addr
		log.WithField("addr", addr.String()).WithField("bytes", n).Info("Packet received")

		l := make([]byte, 2)
		binary.BigEndian.PutUint16(l, uint16(n))
		p := append(l, b[0:n]...)
		log.Printf("payload: %x\n", p)
		if _, err := remote.Write(p); err != nil {
			log.WithError(err).Warn("Could not forward packet.")
		}
	}
}
