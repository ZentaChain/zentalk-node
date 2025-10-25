package network

import (
	"log"
	"net"
	"time"
)

// receiveLoopWithReconnect wraps receiveLoop with automatic reconnection
func (c *Client) receiveLoopWithReconnect() {
	backoff := time.Second
	maxBackoff := 30 * time.Second

	for {
		// Run receive loop
		c.receiveLoop()

		// If explicitly disconnected, don't reconnect
		if !c.connected {
			log.Println("Client disconnected, stopping receive loop")
			return
		}

		// Connection dropped, attempt reconnection
		log.Printf("🔄 Connection lost, reconnecting in %v...", backoff)
		time.Sleep(backoff)

		// Try to reconnect
		if err := c.reconnect(); err != nil {
			log.Printf("❌ Reconnection failed: %v", err)
			// Exponential backoff
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		} else {
			log.Println("✅ Reconnected successfully")
			backoff = time.Second // Reset backoff on success
		}
	}
}

// reconnect attempts to reconnect to the relay
func (c *Client) reconnect() error {
	// Close old connection
	if c.relayConn != nil {
		c.relayConn.Close()
	}

	// Establish new connection
	conn, err := net.Dial("tcp", c.relayAddress)
	if err != nil {
		return err
	}

	c.relayConn = conn

	// Perform handshake
	if err := c.performHandshake(); err != nil {
		conn.Close()
		return err
	}

	return nil
}

// keepaliveLoop sends periodic pings to keep connection alive
func (c *Client) keepaliveLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		<-ticker.C

		if !c.connected {
			return
		}

		if err := c.SendPing(); err != nil {
			log.Printf("⚠️  Keepalive ping failed: %v", err)
		}
	}
}
