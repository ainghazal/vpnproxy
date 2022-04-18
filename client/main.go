package main

import (
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
		remoteConn, err := net.DialTCP("tcp", nil, targetAddr)
		if err != nil {
			log.WithError(err).Fatal("Could not connect to target address:", opts.Target)
			continue
		}
		doProxy(remoteConn, pc)
	}
}

func doProxy(remote net.Conn, local net.PacketConn) {
	// we defer copy remote (tcp) -> local (udp)
	go func() {
		for {
			// Read TCP socket from remote
			b := make([]byte, opts.Buffer)
			n, err := remote.Read(b[:])
			// Write the packet's contents back to the UDP client.
			n, err = local.WriteTo(b[:n], localAddr)
			if err != nil {
				log.Println("error writing")
			}
			/*
				if err != nil {
					doneChan <- err
					return
				}
			*/

			log.Printf("packet-written: bytes=%d to=%s\n", n, localAddr.String())
		}
	}()

	// now we loop-copy local (udp) -> remote (tcp)
	for {
		b := make([]byte, opts.Buffer)
		n, addr, err := local.ReadFrom(b)
		localAddr = addr

		if err != nil {
			log.WithError(err).Error("Could not receive a packet")
			return
		}

		log.WithField("addr", addr.String()).WithField("bytes", n).Info("Packet received")
		if _, err := remote.Write(b[0:n]); err != nil {
			log.WithError(err).Warn("Could not forward packet.")
		}
	}
}
