package message

import (
	"Pororo-droid/go-byshard/identity"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"
)

// Message types for Kademlia protocol
type MessageType int

// Data types
type DataType int

const (
	PING MessageType = iota
	PONG
	FIND_NODE
	FIND_NODE_RESPONSE
	FIND_VALUE
	FIND_VALUE_RESPONSE
	STORE
	STORE_RESPONSE
	BROADCAST
	BROADCAST_ACK
)

// Message represents a Kademlia protocol message
type Message struct {
	Type        MessageType               `json:"type"`
	ID          string                    `json:"id"`
	Sender      identity.KademliaNode     `json:"sender"`
	Target      identity.KademliaNodeID   `json:"target,omitempty"`
	Key         string                    `json:"key,omitempty"`
	Value       string                    `json:"value,omitempty"`
	Nodes       []identity.KademliaNode   `json:"nodes,omitempty"`
	Timestamp   time.Time                 `json:"timestamp"`
	MessageData string                    `json:"message_data,omitempty"` // For broadcast messages
	MessageID   string                    `json:"message_id,omitempty"`   // Unique ID for broadcast messages
	TTL         int                       `json:"ttl,omitempty"`          // Time-to-live for broadcast messages
	SeenBy      []identity.KademliaNodeID `json:"seen_by,omitempty"`      // Nodes that have seen this message
	DataType    string
	Data        interface{}
}

// ShardMessage represents a Kademlia protocol message for Sharding
type ShardMessage struct {
	TargetShard int
	Message     Request
}

// Request represents a client request
type Request struct {
	ClientID  int       `json:"client_id"`
	Operation string    `json:"operation"`
	Target    string    `json: "target"`
	Amount    int       `json:"amount"`
	Condition int       `json:"condition"`
	Timestamp time.Time `json:"timestamp"`
}

func (r Request) GetID() string {
	data, err := json.Marshal(r)
	if err != nil {
		return ""
	}

	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

type ShardRequest struct {
	Votes   []Request
	Commits []Request
	Aborts  []Request
}
