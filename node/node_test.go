package node

import (
	"Pororo-droid/go-byshard/message"
	"Pororo-droid/go-byshard/network"
	"testing"
	"time"
)

func TestPropose(t *testing.T) {
	// Create multiple nodes
	node1 := network.NewKademlia("127.0.0.1", 8001, 1)
	node1.Start()
	defer node1.Stop()

	node2 := NewNode("127.0.0.1", 8002, "pbft", 1)
	node3 := NewNode("127.0.0.1", 8003, "pbft", 1)
	node4 := NewNode("127.0.0.1", 8004, "pbft", 1)
	node5 := NewNode("127.0.0.1", 8005, "pbft", 1)

	go node2.Run()
	go node3.Run()
	go node4.Run()
	go node5.Run()

	req_msg := message.Request{
		ClientID:  1,
		Operation: "test",
		Timestamp: time.Now(),
	}

	node2.Consensus.Propose(req_msg)

	for {

	}
}
