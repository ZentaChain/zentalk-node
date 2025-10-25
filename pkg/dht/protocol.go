package dht

import (
	"fmt"
	"log"
	"sort"
	"sync"
	"time"
)

// Ping sends a PING to a node to check if it's alive
func (n *Node) Ping(target *Contact) error {
	sender := NewContact(n.ID, n.Address)
	req := &PingRequest{}
	msg, err := NewRPCMessage(RPCPing, sender, req)
	if err != nil {
		return err
	}

	response, err := n.sendRPC(target, msg)
	if err != nil {
		return err
	}

	if response.Type != RPCPong {
		return fmt.Errorf("unexpected response type: %v", response.Type)
	}

	// Update routing table
	n.routingTable.AddContact(target)

	return nil
}

// Store stores a key-value pair in the DHT
// Performs iterative node lookup to find k closest nodes, then stores on all of them
func (n *Node) Store(key NodeID, value []byte, ttl time.Duration) error {
	log.Printf("Storing key %s in DHT...", key.String()[:8])

	// Find k closest nodes to the key
	closestNodes := n.iterativeFindNode(key, K)

	if len(closestNodes) == 0 {
		return fmt.Errorf("no nodes available to store key")
	}

	log.Printf("Found %d closest nodes for key %s", len(closestNodes), key.String()[:8])

	// Store on all closest nodes
	sender := NewContact(n.ID, n.Address)
	req := &StoreRequest{
		Key:   key,
		Value: value,
		TTL:   int64(ttl.Seconds()),
	}

	successCount := 0
	for _, contact := range closestNodes {
		msg, err := NewRPCMessage(RPCStore, sender, req)
		if err != nil {
			log.Printf("Failed to create STORE message: %v", err)
			continue
		}

		response, err := n.sendRPC(contact, msg)
		if err != nil {
			log.Printf("Failed to store on node %s: %v", contact.ID.String()[:8], err)
			continue
		}

		if response.Type == RPCStoreAck {
			successCount++
		}
	}

	log.Printf("Successfully stored key %s on %d/%d nodes", key.String()[:8], successCount, len(closestNodes))

	if successCount == 0 {
		return fmt.Errorf("failed to store on any node")
	}

	return nil
}

// Lookup performs iterative value lookup in the DHT
func (n *Node) Lookup(key NodeID) ([]byte, bool) {
	log.Printf("Looking up key %s in DHT...", key.String()[:8])

	// Check local storage first
	if value, found := n.storage.Get(key); found {
		log.Printf("Found key %s in local storage", key.String()[:8])
		return value, true
	}

	// Perform iterative lookup
	return n.iterativeFindValue(key)
}

// iterativeFindNode performs iterative node lookup (core Kademlia algorithm)
// Returns k closest nodes to the target
func (n *Node) iterativeFindNode(target NodeID, k int) []*Contact {
	// Shortlist of nodes we're querying (sorted by distance to target)
	shortlist := n.routingTable.FindClosest(target, k)
	if len(shortlist) == 0 {
		return nil
	}

	// Track nodes we've already queried
	queried := make(map[NodeID]bool)
	queried[n.ID] = true // Don't query ourselves

	// Keep track of closest node seen so far
	var closestSeen *Contact
	if len(shortlist) > 0 {
		closestSeen = shortlist[0]
	}

	sender := NewContact(n.ID, n.Address)

	// Iterative lookup
	for {
		// Find alpha nodes to query (not yet queried)
		toQuery := make([]*Contact, 0, Alpha)
		for _, contact := range shortlist {
			if !queried[contact.ID] && len(toQuery) < Alpha {
				toQuery = append(toQuery, contact)
			}
		}

		// If no new nodes to query, we're done
		if len(toQuery) == 0 {
			break
		}

		// Query alpha nodes in parallel
		var wg sync.WaitGroup
		var mu sync.Mutex
		newContacts := make([]*Contact, 0)

		for _, contact := range toQuery {
			wg.Add(1)
			go func(c *Contact) {
				defer wg.Done()

				// Mark as queried
				mu.Lock()
				queried[c.ID] = true
				mu.Unlock()

				// Send FIND_NODE request
				req := &FindNodeRequest{Target: target}
				msg, err := NewRPCMessage(RPCFindNode, sender, req)
				if err != nil {
					return
				}

				response, err := n.sendRPC(c, msg)
				if err != nil {
					return
				}

				// Parse response
				var resp FindNodeResponse
				if err := ParsePayload(response, &resp); err != nil {
					return
				}

				// Add returned contacts to shortlist
				mu.Lock()
				for _, contact := range resp.Contacts {
					if !queried[contact.ID] && !contact.ID.Equals(n.ID) {
						newContacts = append(newContacts, contact)
						// Update routing table
						n.routingTable.AddContact(contact)
					}
				}
				mu.Unlock()
			}(contact)
		}

		wg.Wait()

		// Add new contacts to shortlist
		shortlist = append(shortlist, newContacts...)

		// Sort by distance to target
		sort.Slice(shortlist, func(i, j int) bool {
			return shortlist[i].ID.CloserTo(target, shortlist[j].ID)
		})

		// Keep only k closest
		if len(shortlist) > k {
			shortlist = shortlist[:k]
		}

		// Check if we've found a closer node
		if len(shortlist) > 0 {
			if closestSeen == nil || shortlist[0].ID.CloserTo(target, closestSeen.ID) {
				closestSeen = shortlist[0]
			} else {
				// No closer node found - terminate
				break
			}
		}
	}

	return shortlist
}

// iterativeFindValue performs iterative value lookup
func (n *Node) iterativeFindValue(key NodeID) ([]byte, bool) {
	// Similar to iterativeFindNode but uses FIND_VALUE instead
	shortlist := n.routingTable.FindClosest(key, K)
	if len(shortlist) == 0 {
		return nil, false
	}

	queried := make(map[NodeID]bool)
	queried[n.ID] = true

	sender := NewContact(n.ID, n.Address)

	for {
		toQuery := make([]*Contact, 0, Alpha)
		for _, contact := range shortlist {
			if !queried[contact.ID] && len(toQuery) < Alpha {
				toQuery = append(toQuery, contact)
			}
		}

		if len(toQuery) == 0 {
			break
		}

		// Query alpha nodes in parallel
		type result struct {
			value    []byte
			found    bool
			contacts []*Contact
		}

		resultChan := make(chan result, len(toQuery))
		var wg sync.WaitGroup

		for _, contact := range toQuery {
			wg.Add(1)
			go func(c *Contact) {
				defer wg.Done()

				queried[c.ID] = true

				// Send FIND_VALUE request
				req := &FindValueRequest{Key: key}
				msg, err := NewRPCMessage(RPCFindValue, sender, req)
				if err != nil {
					return
				}

				response, err := n.sendRPC(c, msg)
				if err != nil {
					return
				}

				// Parse response
				var resp FindValueResponse
				if err := ParsePayload(response, &resp); err != nil {
					return
				}

				resultChan <- result{
					value:    resp.Value,
					found:    resp.Found,
					contacts: resp.Contacts,
				}
			}(contact)
		}

		wg.Wait()
		close(resultChan)

		// Process results
		newContacts := make([]*Contact, 0)
		for res := range resultChan {
			if res.found {
				// Value found!
				log.Printf("Found key %s in DHT", key.String()[:8])
				return res.value, true
			}

			// Add new contacts
			for _, contact := range res.contacts {
				if !queried[contact.ID] && !contact.ID.Equals(n.ID) {
					newContacts = append(newContacts, contact)
					n.routingTable.AddContact(contact)
				}
			}
		}

		if len(newContacts) == 0 {
			break
		}

		shortlist = append(shortlist, newContacts...)
		sort.Slice(shortlist, func(i, j int) bool {
			return shortlist[i].ID.CloserTo(key, shortlist[j].ID)
		})

		if len(shortlist) > K {
			shortlist = shortlist[:K]
		}
	}

	log.Printf("Key %s not found in DHT", key.String()[:8])
	return nil, false
}

// Bootstrap joins the DHT network by contacting a bootstrap node
func (n *Node) Bootstrap(bootstrapNode *Contact) error {
	log.Printf("Bootstrapping DHT from node %s...", bootstrapNode.ID.String()[:8])

	// Add bootstrap node to routing table
	n.routingTable.AddContact(bootstrapNode)

	// Perform node lookup for our own ID to populate routing table
	closestNodes := n.iterativeFindNode(n.ID, K)

	log.Printf("Bootstrap complete: discovered %d nodes", len(closestNodes))

	return nil
}

// AddPeer manually adds a peer to the routing table
func (n *Node) AddPeer(contact *Contact) {
	n.routingTable.AddContact(contact)
}
