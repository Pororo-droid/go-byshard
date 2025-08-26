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

	shard3_ip_list := []string{"127.0.0.1", "127.0.0.1", "127.0.0.1", "127.0.0.1"}
	shard3_port_list := []int{8012, 8013, 8014, 8015}

	var nodes_list [][]node.Node

	nodes_list = append(nodes_list, Start(shard1_ip_list, shard1_port_list, 1))
	nodes_list = append(nodes_list, Start(shard2_ip_list, shard2_port_list, 2))
	nodes_list = append(nodes_list, Start(shard3_ip_list, shard3_port_list, 3))

	propose(nodes_list[0])
	for {
	}
}

func Start(ip_list []string, port_list []int, shard_num int) []node.Node {
	bootstrap_node := network.NewKademlia(config.GetConfig().BootstrapList[shard_num-1].IP, config.GetConfig().BootstrapList[shard_num-1].Port, shard_num)
	bootstrap_node.Start()
	// defer bootstrap_node.Stop()

	nodes := make([]node.Node, len(ip_list))

	for i := range ip_list {
		new_node := node.NewNode(ip_list[i], port_list[i], "pbft", shard_num)
		nodes[i] = new_node
	}

	for i := 0; i < len(nodes); i++ {
		go nodes[i].Run()
	}

	// req_msg := message.Request{
	// 	ClientID:  1,
	// 	Operation: "test",
	// 	Timestamp: time.Now(),
	// }

	nodes[0].Consensus.SetToPrimary()
	// nodes[0].Consensus.Propose(req_msg)
	nodes[0].Orchestration.SetToPrimary()

	return nodes
}

func propose(nodes []node.Node) {
	shard_req := message.ShardRequest{
		Votes: []message.Request{
			message.Request{
				ClientID:  1,
				Operation: "Sub",
				Target:    "Ana",
				Amount:    400,
				Condition: 500,
				Timestamp: time.Now(),
			},
			message.Request{
				ClientID:  1,
				Operation: "Sub",
				Target:    "Bo",
				Amount:    100,
				Condition: 200,
				Timestamp: time.Now(),
			},
		}, Commits: []message.Request{
			message.Request{
				ClientID:  1,
				Operation: "Add",
				Target:    "Carol",
				Amount:    500,
				Condition: 0,
				Timestamp: time.Now(),
			}, message.Request{
				ClientID:  1,
				Operation: "Add",
				Target:    "Ana",
				Amount:    100,
				Condition: 200,
				Timestamp: time.Now(),
			},
		}, Aborts: []message.Request{
			message.Request{
				ClientID:  1,
				Operation: "Add",
				Target:    "Ana",
				Amount:    400,
				Condition: 0,
				Timestamp: time.Now(),
			},
		},
	}

	nodes[0].Orchestration.Propose(shard_req)
}
