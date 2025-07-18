package network

import (
	"Pororo-droid/go-byshard/message"
	"fmt"
	"testing"
	"time"
)

func TestNetwork(t *testing.T) {
	// Create multiple nodes
	node1 := NewKademlia("127.0.0.1", 8001, 1)
	node2 := NewKademlia("127.0.0.1", 8002, 1)
	node3 := NewKademlia("127.0.0.1", 8003, 1)
	node4 := NewKademlia("127.0.0.1", 8004, 1)
	node5 := NewKademlia("127.0.0.1", 8005, 1)

	// Start all nodes
	nodes := []*Kademlia{node1, node2, node3, node4, node5}
	for i, node := range nodes {
		if err := node.Start(); err != nil {
			fmt.Printf("Error starting node%d: %v\n", i+1, err)
			return
		}
		defer node.Stop()
		fmt.Printf("노드 %d (포트 %d) 시작됨\n", i+1, 8001+i)
	}

	// Give nodes time to start
	time.Sleep(300 * time.Millisecond)

	fmt.Println("\n=== 네트워크 구축 과정 ===")

	// Node2 joins through node1 (bootstrap)
	fmt.Println("\n--- 노드2가 노드1을 통해 네트워크 참여 ---")
	if err := node2.Join(node1.GetNodeInfo()); err != nil {
		fmt.Printf("Error node2 joining network: %v\n", err)
		return
	}
	time.Sleep(500 * time.Millisecond)

	// Node3 joins through node1 (should get introduced to node2)
	fmt.Println("\n--- 노드3이 노드1을 통해 네트워크 참여 ---")
	if err := node3.Join(node1.GetNodeInfo()); err != nil {
		fmt.Printf("Error node3 joining network: %v\n", err)
		return
	}
	time.Sleep(500 * time.Millisecond)

	// Node4 joins through node2 (should get introduced to node1 and node3)
	fmt.Println("\n--- 노드4가 노드2를 통해 네트워크 참여 ---")
	if err := node4.Join(node2.GetNodeInfo()); err != nil {
		fmt.Printf("Error node4 joining network: %v\n", err)
		return
	}
	time.Sleep(500 * time.Millisecond)

	// Node5 joins through node3 (should get introduced to all others)
	fmt.Println("\n--- 노드5가 노드3을 통해 네트워크 참여 ---")
	if err := node5.Join(node3.GetNodeInfo()); err != nil {
		fmt.Printf("Error node5 joining network: %v\n", err)
		return
	}
	time.Sleep(1 * time.Second)

	// Print final network topology
	fmt.Println("\n=== 최종 네트워크 토폴로지 ===")
	for i, node := range nodes {
		connectedNodes := node.GetConnectedNodes()
		fmt.Printf("노드 %d (포트 %d): %d개 노드 연결", i+1, 8001+i, len(connectedNodes))
		if len(connectedNodes) > 0 {
			fmt.Printf(" -> 연결된 포트들: ")
			for j, connectedNode := range connectedNodes {
				if j > 0 {
					fmt.Printf(", ")
				}
				fmt.Printf("%d", connectedNode.Port)
			}
		}
		fmt.Println()
	}

	// Store a value from node1
	fmt.Println("\n=== 값 저장 및 검색 테스트 ===")
	if err := node1.Store("hello", "world from distributed network"); err != nil {
		fmt.Printf("Error storing value: %v\n", err)
		return
	}
	fmt.Println("노드1에서 'hello' -> 'world from distributed network' 저장 완료")

	// Try to find the value from node5 (should work through the network)
	fmt.Println("\n--- 노드5에서 값 검색 ---")
	value, err := node5.FindValue("hello")
	if err != nil {
		fmt.Printf("노드5에서 값 검색 실패: %v\n", err)
	} else {
		fmt.Printf("노드5에서 'hello' 검색 결과: %s\n", value)
	}

	// Test broadcast functionality
	fmt.Println("\n=== 브로드캐스트 테스트 ===")
	time.Sleep(500 * time.Millisecond)

	// Node1 broadcasts a message
	fmt.Println("\n--- 노드1 브로드캐스트 ---")
	msg := makeBroadcastMessage(node1.node.ID.String(), "안녕하세요! 노드1에서 보내는 네트워크 전체 메시지입니다.")
	if err := node1.Broadcast(msg); err != nil {
		fmt.Printf("Error broadcasting from node1: %v\n", err)
	}

	time.Sleep(1 * time.Second)

	// Node5 broadcasts a message
	fmt.Println("\n--- 노드5 브로드캐스트 ---")
	msg = makeBroadcastMessage(node5.node.ID.String(), "노드5에서 전송하는 분산 네트워크 테스트 메시지!")
	if err := node5.Broadcast(msg); err != nil {
		fmt.Printf("Error broadcasting from node5: %v\n", err)
	}

	// Keep running to see all results
	time.Sleep(3 * time.Second)

	fmt.Println("\n=== 모든 테스트 완료 ===")
}

func makeBroadcastMessage(nodeID string, msg string) message.Message {
	messageID := fmt.Sprintf("broadcast-%s-%d", nodeID[:8], time.Now().UnixNano())
	return message.Message{
		Type:        message.BROADCAST,
		ID:          fmt.Sprintf("broadcast-init-%d", time.Now().UnixNano()),
		MessageData: msg,
		MessageID:   messageID,
		TTL:         5, // Maximum 5 hops
		Timestamp:   time.Now(),
	}
}
