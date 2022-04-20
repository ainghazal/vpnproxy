package main

import (
	"bufio"
	"encoding/binary"
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
		go doProxy(conn, remoteConn)
	}
}

func doProxy(local net.Conn, remote *net.UDPConn) {
	// copy remote (udp) -> local (tcp)
	go func() {
		for {
			b := make([]byte, opts.Buffer)
			n, addr, err := remote.ReadFromUDP(b[:])
			if err != nil {
				log.WithError(err).Error("Could not receive a packet")
				return
			}

			log.WithField("addr", addr.String()).WithField("bytes", n).WithField("payload", string(b[:n])).Info("Packet received")

			l := make([]byte, 2)
			binary.BigEndian.PutUint16(l, uint16(n))
			p := append(l, b[0:n]...)

			if _, err := local.Write(p); err != nil {
				log.WithError(err).Warn("Could not forward packet.")
			}
		}
	}()

	// copy local (tcp) -> remote (udp)
	writer := bufio.NewWriter(remote)
	bufLen := make([]byte, 2)
	for {
		// Simple framing:
		// Read 2 bytes from TCP stream, update bufLen
		n, err := local.Read(bufLen)
		if err != nil {
			log.WithError(err).Error("Could not read len from local TCP connection")
		}
		l := binary.BigEndian.Uint16(bufLen)

		// Now let's allocate exactly the bytes we need
		b := make([]byte, l)
		n, err = local.Read(b[:])
		if err != nil {
			log.WithError(err).Error("Could not read payload from local TCP connection")
		}
		// Write the packet's contents to the remote UDP conn.
		writer.Write(b[:])
		err = writer.Flush()
		if err != nil {
			log.WithError(err).Error("Could not write")
		}

		log.Printf("packet-written: bytes=%d to=%s\n", n, opts.Target)
		if n > 1500 {
			log.Printf("%x\n", b[:n])
		}
	}
}
