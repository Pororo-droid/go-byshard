package main

import (
	"Pororo-droid/go-byshard/config"
	"Pororo-droid/go-byshard/log"
	"Pororo-droid/go-byshard/message"
	"Pororo-droid/go-byshard/network"
	"Pororo-droid/go-byshard/node"
	"time"
)

// Example usage
func main() {
	log.Setup()

	shard1_ip_list := []string{"127.0.0.1", "127.0.0.1", "127.0.0.1", "127.0.0.1"}
	shard1_port_list := []int{8002, 8003, 8004, 8005}

	shard2_ip_list := []string{"127.0.0.1", "127.0.0.1", "127.0.0.1", "127.0.0.1"}
	shard2_port_list := []int{8007, 8008, 8009, 8010}

	Start(shard1_ip_list, shard1_port_list, 1)
	Start(shard2_ip_list, shard2_port_list, 2)

	for {
	}
}

func Start(ip_list []string, port_list []int, shard_num int) {
	// Create multiple nodes
	var node1 *network.Kademlia
	if shard_num == 1 {
		node1 = network.NewKademlia(config.GetConfig().Shard1.Bootstrap.IP, config.GetConfig().Shard1.Bootstrap.Port, shard_num)
	} else if shard_num == 2 {
		node1 = network.NewKademlia(config.GetConfig().Shard2.Bootstrap.IP, config.GetConfig().Shard2.Bootstrap.Port, shard_num)
	}
	node1.Start()
	defer node1.Stop()

	nodes := make([]node.Node, len(ip_list))

	for i := range ip_list {
		new_node := node.NewNode(ip_list[i], port_list[i], "pbft", shard_num)
		nodes[i] = new_node
	}

	for i := 0; i < len(nodes); i++ {
		go nodes[i].Run()
	}

	req_msg := message.Request{
		ClientID:  1,
		Operation: "test",
		Timestamp: time.Now(),
	}

	nodes[0].Consensus.SetToPrimary()
	nodes[0].Consensus.Propose(req_msg)
}
