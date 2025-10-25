package network

import (
	"io"
	"log"
	"net"

	"github.com/ZentaChain/zentalk-node/pkg/protocol"
)

// acceptLoop accepts incoming connections
func (rs *RelayServer) acceptLoop() {
	for {
		conn, err := rs.listener.Accept()
		if err != nil {
			log.Printf("Accept error: %v", err)
			return
		}

		go rs.handleConnection(conn)
	}
}

// handleConnection handles a peer connection
func (rs *RelayServer) handleConnection(conn net.Conn) {
	defer conn.Close()

	log.Printf("New connection from %s", conn.RemoteAddr())

	var peerAddr protocol.Address

	// Cleanup peer on disconnect
	defer func() {
		if peerAddr != (protocol.Address{}) {
			rs.mu.Lock()
			delete(rs.peers, string(peerAddr[:]))
			rs.mu.Unlock()
			log.Printf("Peer disconnected and removed: %x", peerAddr[:8])
		}
	}()

	// Loop to handle multiple messages on same connection
	for {
		// Read and validate header
		header, err := protocol.ReadHeader(conn)
		if err != nil {
			if err != io.EOF {
				log.Printf("Header error: %v", err)
			}
			return
		}

		// Handle message based on type
		switch header.Type {
		case protocol.MsgTypeHandshake:
			peerAddr = rs.handleHandshake(conn, header)

		case protocol.MsgTypeRelayForward:
			rs.handleRelayForward(conn, header)

		case protocol.MsgTypePing:
			rs.handlePing(conn, header)

		default:
			log.Printf("Unknown message type: 0x%04x", header.Type)
		}
	}
}
