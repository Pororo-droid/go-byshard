package network

import (
	"Pororo-droid/go-byshard/identity"
	"math/big"
	"sort"
	"sync"
	"time"
)

// Contact represents a contact in the routing table
type Contact struct {
	Node     identity.KademliaNode
	LastSeen time.Time
}

// RoutingTable represents the Kademlia routing table
type RoutingTable struct {
	localNode identity.KademliaNode
	buckets   [ID_LENGTH]*Bucket
	mutex     sync.RWMutex
}

// NewRoutingTable creates a new routing table
func NewRoutingTable(localNode identity.KademliaNode) *RoutingTable {
	rt := &RoutingTable{
		localNode: localNode,
	}

	for i := 0; i < ID_LENGTH; i++ {
		rt.buckets[i] = NewBucket()
	}

	return rt
}

// getBucketIndex calculates the bucket index for a given node
func (rt *RoutingTable) getBucketIndex(nodeID identity.KademliaNodeID) int {
	distance := rt.localNode.ID.XOR(nodeID)
	if distance.Cmp(big.NewInt(0)) == 0 {
		return 0
	}

	// Find the position of the most significant bit
	bitLength := distance.BitLen()
	if bitLength == 0 {
		return 0
	}

	bucketIndex := ID_LENGTH - bitLength
	// Ensure bucket index is within valid range
	if bucketIndex < 0 {
		bucketIndex = 0
	}
	if bucketIndex >= ID_LENGTH {
		bucketIndex = ID_LENGTH - 1
	}

	return bucketIndex
}

// AddContact adds a contact to the routing table
func (rt *RoutingTable) AddContact(node identity.KademliaNode) {
	if node.ID == rt.localNode.ID {
		return
	}

	bucketIndex := rt.getBucketIndex(node.ID)
	contact := Contact{Node: node, LastSeen: time.Now()}
	rt.buckets[bucketIndex].AddContact(contact)
}

// FindClosestNodes finds the k closest nodes to a target ID
func (rt *RoutingTable) FindClosestNodes(target identity.KademliaNodeID, k int) []identity.KademliaNode {
	var allContacts []Contact

	// Collect all contacts from all buckets
	for _, bucket := range rt.buckets {
		allContacts = append(allContacts, bucket.GetContacts()...)
	}

	// Sort by distance to target
	sort.Slice(allContacts, func(i, j int) bool {
		distI := allContacts[i].Node.ID.XOR(target)
		distJ := allContacts[j].Node.ID.XOR(target)
		return distI.Cmp(distJ) < 0
	})

	// Return up to k closest nodes
	var result []identity.KademliaNode
	for i, contact := range allContacts {
		if i >= k {
			break
		}
		result = append(result, contact.Node)
	}

	return result
}

// RemoveContact removes a contact from the routing table
func (rt *RoutingTable) RemoveContact(nodeID identity.KademliaNodeID) {
	if nodeID == rt.localNode.ID {
		return
	}

	bucketIndex := rt.getBucketIndex(nodeID)
	rt.buckets[bucketIndex].RemoveContact(nodeID)
}
