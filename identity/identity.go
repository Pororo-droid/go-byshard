package identity

import (
	"crypto/sha1"
	"fmt"
	"math/big"
)

// NodeID represents a 160-bit node identifier
type KademliaNodeID [20]byte

// Node represents a Kademlia node
type KademliaNode struct {
	ID   KademliaNodeID
	IP   string
	Port int
}

// NewNodeID creates a new NodeID from a string
func NewNodeID(data string) KademliaNodeID {
	hash := sha1.Sum([]byte(data))
	return KademliaNodeID(hash)
}

// XOR calculates XOR distance between two NodeIDs
func (n KademliaNodeID) XOR(other KademliaNodeID) *big.Int {
	result := new(big.Int)
	a := new(big.Int).SetBytes(n[:])
	b := new(big.Int).SetBytes(other[:])
	result.Xor(a, b)
	return result
}

// String returns string representation of NodeID
func (n KademliaNodeID) String() string {
	return fmt.Sprintf("%x", n[:])
}
