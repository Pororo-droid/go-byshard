package main

import (
	"Pororo-droid/go-byshard/log"
	"Pororo-droid/go-byshard/message"
	"Pororo-droid/go-byshard/network"
	"Pororo-droid/go-byshard/node"
	"time"
)

// Example usage
func main() {
	log.Setup()
	// Create multiple nodes
	node1 := network.NewKademlia("127.0.0.1", 8001)
	node1.Start()
	defer node1.Stop()

	node2 := node.NewNode("127.0.0.1", 8002, "pbft")
	node3 := node.NewNode("127.0.0.1", 8003, "pbft")
	node4 := node.NewNode("127.0.0.1", 8004, "pbft")
	node5 := node.NewNode("127.0.0.1", 8005, "pbft")

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
