package network

import (
	"Pororo-droid/go-byshard/config"
	"Pororo-droid/go-byshard/identity"
	"Pororo-droid/go-byshard/log"
	"Pororo-droid/go-byshard/message"
	"encoding/json"
	"fmt"
	"net"
	"sort"
	"sync"
	"time"
)

// KademliaNode represents a Kademlia DHT node
type Kademlia struct {
	node identity.KademliaNode
	// routingTable *RoutingTable
	routingTable map[int]*RoutingTable
	storage      map[string]string
	listener     net.Listener
	mutex        sync.RWMutex
	running      bool
	seenMessages map[string]bool // Track seen broadcast messages
	messageMutex sync.RWMutex    // Mutex for seen messages

	ConsensusMessages chan message.Message
	ShardMessages     chan message.Message

	Bootstrap identity.KademliaNode
	ShardNum  int
}

// func getBootstrap() identity.KademliaNode {
// 	var bootstrap identity.KademliaNode

// 	bootstrap.IP = config.GetBootstrapConfig().IP
// 	bootstrap.Port = config.GetBootstrapConfig().Port
// 	bootstrap.ID = identity.NewNodeID(fmt.Sprintf("%s:%d", bootstrap.IP, bootstrap.Port))

// 	return bootstrap
// }

// NewKademlia creates a new Kademlia Table and connection
func NewKademlia(ip string, port int, shard_num int) *Kademlia {
	nodeID := identity.NewNodeID(fmt.Sprintf("%s:%d", ip, port))
	node := identity.KademliaNode{ID: nodeID, IP: ip, Port: port}

	bootstrap := identity.KademliaNode{
		IP:   config.GetConfig().BootstrapList[shard_num-1].IP,
		Port: config.GetConfig().BootstrapList[shard_num-1].Port,
		ID:   identity.NewNodeID(fmt.Sprintf("%s:%d", config.GetConfig().BootstrapList[shard_num-1].IP, config.GetConfig().BootstrapList[shard_num-1].Port)),
	}

	routingTable := make(map[int]*RoutingTable)
	routingTable[shard_num] = NewRoutingTable(node)
	return &Kademlia{
		node: node,
		// routingTable:      NewRoutingTable(node),
		routingTable:      routingTable,
		storage:           make(map[string]string),
		running:           false,
		seenMessages:      make(map[string]bool),
		ConsensusMessages: make(chan message.Message, config.GetConfig().ChanelSize),
		ShardMessages:     make(chan message.Message, config.GetConfig().ChanelSize),
		Bootstrap:         bootstrap,
		ShardNum:          shard_num,
	}
}

// func getBootstrap() identity.KademliaNode {
// 	var bootstrap identity.KademliaNode

// 	bootstrap.IP = config.GetBootstrapConfig().IP
// 	bootstrap.Port = config.GetBootstrapConfig().Port
// 	bootstrap.ID = identity.NewNodeID(fmt.Sprintf("%s:%d", bootstrap.IP, bootstrap.Port))

// 	return bootstrap
// }

// Sets Kademlia node
func (kn *Kademlia) Setup() error {
	if err := kn.Start(); err != nil {
		return err
	}

	// if err := kn.Join(kn.Bootstrap); err != nil {
	// 	return err
	// }

	if err := kn.Join(kn.ShardNum); err != nil {
		return err
	}

	return nil
}

// Start starts the Kademlia node
func (kn *Kademlia) Start() error {
	addr := fmt.Sprintf("%s:%d", kn.node.IP, kn.node.Port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	kn.listener = listener
	kn.running = true

	fmt.Printf("Kademlia node started on %s\n", addr)

	go kn.handleConnections()
	return nil
}

// Stop stops the Kademlia node
func (kn *Kademlia) Stop() {
	kn.running = false
	if kn.listener != nil {
		kn.listener.Close()
	}

	// Close channels to signal shutdown
	close(kn.ConsensusMessages)
	close(kn.ShardMessages)

	log.Infof("Kademlia node %s:%d stopped", kn.node.IP, kn.node.Port)
}

// handleConnections handles incoming TCP connections
func (kn *Kademlia) handleConnections() {
	for kn.running {
		conn, err := kn.listener.Accept()
		if err != nil {
			if kn.running {
				fmt.Printf("Error accepting connection: %v\n", err)
			}
			continue
		}

		go kn.handleConnection(conn)
	}
}

// handleConnection handles a single TCP connection
func (kn *Kademlia) handleConnection(conn net.Conn) {
	defer conn.Close()

	decoder := json.NewDecoder(conn)
	encoder := json.NewEncoder(conn)

	var msg message.Message
	if err := decoder.Decode(&msg); err != nil {
		fmt.Printf("Error decoding message: %v\n", err)
		return
	}

	// Add sender to routing table
	// kn.routingTable.AddContact(msg.Sender)
	// kn.routingTable[msg.Shard].AddContact(msg.Sender)
	if _, exists := kn.routingTable[msg.SenderShard]; !exists {
		kn.routingTable[msg.SenderShard] = NewRoutingTable(kn.node)
	}
	kn.routingTable[msg.SenderShard].AddContact(msg.Sender)

	response := kn.handleMessage(msg)
	if response != nil {
		if err := encoder.Encode(response); err != nil {
			fmt.Printf("Error encoding response: %v\n", err)
		}
	}
}

// handleMessage processes incoming messages
func (kn *Kademlia) handleMessage(msg message.Message) *message.Message {
	switch msg.Type {
	case message.PING:
		return &message.Message{
			Type:      message.PONG,
			ID:        msg.ID,
			Sender:    kn.node,
			Timestamp: time.Now(),
			// Shard:     msg.Shard,
			TargetShard: msg.TargetShard,
			SenderShard: msg.SenderShard,
		}

	case message.FIND_NODE:
		// nodes := kn.routingTable.FindClosestNodes(msg.Target, K)
		// return &message.Message{
		// 	Type:      message.FIND_NODE_RESPONSE,
		// 	ID:        msg.ID,
		// 	Sender:    kn.node,
		// 	Nodes:     nodes,
		// 	Timestamp: time.Now(),
		// }
		nodes := kn.routingTable[msg.TargetShard].FindClosestNodes(msg.Target, K)
		return &message.Message{
			Type:      message.FIND_NODE_RESPONSE,
			ID:        msg.ID,
			Sender:    kn.node,
			Nodes:     nodes,
			Timestamp: time.Now(),
			// Shard:     msg.Shard,
			TargetShard: msg.TargetShard,
			SenderShard: msg.SenderShard,
		}

	case message.BROADCAST:
		// Handle broadcast message
		kn.messageMutex.Lock()
		if kn.seenMessages[msg.MessageID] {
			// Already seen this message, ignore
			// fmt.Printf("[노드 %s:%d] 브로드캐스트 메시지 수신: '%s' (발신자: %s:%d)\n",
			// 	kn.node.IP, kn.node.Port, msg.ID, msg.Sender.IP, msg.Sender.Port)

			kn.messageMutex.Unlock()
			return &message.Message{
				Type:      message.BROADCAST_ACK,
				ID:        msg.ID,
				Sender:    kn.node,
				Timestamp: time.Now(),
				// Shard:     msg.Shard,
				TargetShard: msg.TargetShard,
				SenderShard: msg.SenderShard,
			}
		}

		// Mark message as seen
		kn.seenMessages[msg.MessageID] = true
		kn.messageMutex.Unlock()

		// Print received message
		// fmt.Printf("[노드 %s:%d] 브로드캐스트 메시지 수신: '%s' (발신자: %s:%d)\n",
		// 	kn.node.IP, kn.node.Port, msg.MessageData, msg.Sender.IP, msg.Sender.Port)\
		kn.putData(msg)
		// Forward to other nodes if TTL > 0
		if msg.TTL > 0 {
			go kn.forwardBroadcast(msg)
		}

		return &message.Message{
			Type:      message.BROADCAST_ACK,
			ID:        msg.ID,
			Sender:    kn.node,
			Timestamp: time.Now(),
			// Shard:     msg.Shard,
			TargetShard: msg.TargetShard,
			SenderShard: msg.SenderShard,
		}

	case message.LEAVE:
		// Handle leave message - remove the sender from our routing table
		// log.Infof("Node %s:%d received leave notification from %s:%d", kn.node.IP, kn.node.Port, msg.Sender.IP, msg.Sender.Port)

		// kn.routingTable.RemoveContact(msg.Sender.ID)
		kn.routingTable[msg.SenderShard].RemoveContact(msg.Sender.ID)

		return &message.Message{
			Type:      message.LEAVE_ACK,
			ID:        msg.ID,
			Sender:    kn.node,
			Timestamp: time.Now(),
			// Shard:     msg.Shard,
			TargetShard: msg.TargetShard,
			SenderShard: msg.SenderShard,
		}
	}
	return nil
}

// sendMessage sends a message to a remote node
func (kn *Kademlia) sendMessage(target identity.KademliaNode, msg message.Message) (*message.Message, error) {
	addr := fmt.Sprintf("%s:%d", target.IP, target.Port)
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	encoder := json.NewEncoder(conn)
	decoder := json.NewDecoder(conn)

	msg.Sender = kn.node
	msg.Timestamp = time.Now()

	// checkShard
	if msg.TargetShard == 0 || msg.SenderShard == 0 {
		panic("shard 0")
	}

	if err := encoder.Encode(msg); err != nil {
		return nil, err
	}

	var response message.Message
	if err := decoder.Decode(&response); err != nil {
		return nil, err
	}

	// Add responder to routing table
	// kn.routingTable.AddContact(response.Sender)
	kn.routingTable[msg.SenderShard].AddContact(response.Sender)

	return &response, nil
}

// Ping sends a ping message to a node
func (kn *Kademlia) Ping(target identity.KademliaNode, shard_num int) error {
	msg := message.Message{
		Type: message.PING,
		ID:   fmt.Sprintf("ping-%d", time.Now().UnixNano()),
		// Shard: shard_num,
		TargetShard: shard_num,
		SenderShard: kn.ShardNum,
	}

	response, err := kn.sendMessage(target, msg)
	if err != nil {
		return err
	}

	if response.Type != message.PONG {
		return fmt.Errorf("unexpected response type: %v", response.Type)
	}

	return nil
}

// FindNode finds nodes close to a target ID
func (kn *Kademlia) FindNode(target identity.KademliaNodeID, shard_num int) ([]identity.KademliaNode, error) {
	msg := message.Message{
		Type:   message.FIND_NODE,
		ID:     fmt.Sprintf("find_node-%d", time.Now().UnixNano()),
		Target: target,
		// Shard:  shard_num,
		TargetShard: shard_num,
		SenderShard: kn.ShardNum,
	}

	closestNodes := kn.routingTable[shard_num].FindClosestNodes(target, ALPHA)
	if len(closestNodes) == 0 {
		return []identity.KademliaNode{}, nil // Return empty slice instead of error
	}

	var allNodes []identity.KademliaNode
	for _, node := range closestNodes {
		response, err := kn.sendMessage(node, msg)
		if err != nil {
			// fmt.printf("Error sending find_node message to %s: %v\n", node.ID.String(), err)
			continue
		}

		if response.Type == message.FIND_NODE_RESPONSE {
			allNodes = append(allNodes, response.Nodes...)
		}
	}

	return allNodes, nil
}

// Join joins the Kademlia network through a bootstrap node
func (kn *Kademlia) Join(shard_num int) error {
	// fmt.printf("[노드 %s:%d] 네트워크 참여 시작 - bootstrap: %s:%d\n",
	//	 kn.node.IP, kn.node.Port, bootstrap.IP, bootstrap.Port)
	if _, exists := kn.routingTable[shard_num]; !exists {
		kn.routingTable[shard_num] = NewRoutingTable(kn.node)
	}

	bootstrap := identity.KademliaNode{
		IP:   config.GetConfig().BootstrapList[shard_num-1].IP,
		Port: config.GetConfig().BootstrapList[shard_num-1].Port,
		ID:   identity.NewNodeID(fmt.Sprintf("%s:%d", config.GetConfig().BootstrapList[shard_num-1].IP, config.GetConfig().BootstrapList[shard_num-1].Port)),
	}

	// Step 1: Add bootstrap node to routing table
	kn.routingTable[shard_num].AddContact(bootstrap)

	// Step 2: Ping bootstrap node to establish connection
	if err := kn.Ping(bootstrap, shard_num); err != nil {
		return fmt.Errorf("failed to ping bootstrap node: %v", err)
	}
	// Step 3: Ask bootstrap for nodes close to our ID
	// fmt.printf("[노드 %s:%d] bootstrap에게 근처 노드들 요청\n", kn.node.IP, kn.node.Port)
	nearbyNodes, err := kn.FindNode(kn.node.ID, shard_num)
	if err != nil {
		return fmt.Errorf("[노드 %s:%d] bootstrap으로부터 근처 노드 찾기 실패: %v",
			kn.node.IP, kn.node.Port, err)
	}

	// Step 4: Connect to all nodes returned by bootstrap
	connectedCount := 0
	for _, node := range nearbyNodes {
		if node.ID == kn.node.ID {
			continue // Skip ourselves
		}

		// Add to routing table
		kn.routingTable[shard_num].AddContact(node)

		// Try to ping each node
		if err := kn.Ping(node, shard_num); err != nil {
			return fmt.Errorf("[노드 %s:%d] %s:%d 연결 실패: %v",
				kn.node.IP, kn.node.Port, node.IP, node.Port, err)
		} else {
			connectedCount++
		}
	}

	// Step 5: Perform iterative node lookup to discover more nodes
	// fmt.Printf("[노드 %s:%d] 반복적 노드 탐색 시작\n", kn.node.IP, kn.node.Port)
	if err := kn.performIterativeNodeLookup(shard_num); err != nil {
		return fmt.Errorf("[노드 %s:%d] 반복적 노드 탐색 실패: %v",
			kn.node.IP, kn.node.Port, err)
	}

	// totalConnected := kn.GetNodeCount()
	// fmt.Printf("[노드 %s:%d] 네트워크 참여 완료 - 총 %d개 노드 연결\n",
	// 	kn.node.IP, kn.node.Port, totalConnected)

	// log.Infof("[%v:%v]Join completed to %v shard, connected node on shard num: %v, shard: %v", kn.node.IP, kn.node.Port, shard_num, len(kn.GetConnectedNodes(shard_num)), kn.GetConnectedNodes(shard_num))

	return nil
}

// GetID returns the node's ID
func (kn *Kademlia) GetID() identity.KademliaNodeID {
	return kn.node.ID
}

// GetNodeInfo returns the node's information
func (kn *Kademlia) GetNodeInfo() identity.KademliaNode {
	return kn.node
}

// performIterativeNodeLookup performs iterative node lookup to populate routing table
func (kn *Kademlia) performIterativeNodeLookup(shard_num int) error {
	// Generate random target IDs to explore different parts of the network
	targets := []identity.KademliaNodeID{
		kn.node.ID, // Look for nodes close to ourselves
	}

	// Add some random targets to explore the network
	for i := 0; i < 3; i++ {
		randomTarget := identity.NewNodeID(fmt.Sprintf("random-%d-%d", i, time.Now().UnixNano()))
		targets = append(targets, randomTarget)
	}

	for _, target := range targets {
		nodes, err := kn.iterativeFindNode(target, shard_num)
		if err != nil {
			continue
		}

		// Connect to discovered nodes
		for _, node := range nodes {
			if node.ID != kn.node.ID {
				kn.routingTable[shard_num].AddContact(node)
				// Try to establish connection
				go func(n identity.KademliaNode) {
					// if err := kn.Ping(n); err == nil {
					// 	fmt.Printf("[노드 %s:%d] 탐색을 통해 %s:%d 연결\n",
					// 		kn.node.IP, kn.node.Port, n.IP, n.Port)
					// }
					kn.Ping(n, shard_num)
				}(node)
			}
		}
	}

	return nil
}

// iterativeFindNode performs iterative FIND_NODE lookup
func (kn *Kademlia) iterativeFindNode(target identity.KademliaNodeID, shard_num int) ([]identity.KademliaNode, error) {
	// Get initial candidates from our routing table
	candidates := kn.routingTable[shard_num].FindClosestNodes(target, K)
	if len(candidates) == 0 {
		return nil, fmt.Errorf("no nodes in routing table")
	}

	queried := make(map[identity.KademliaNodeID]bool)

	// Iteratively query ALPHA closest unqueried nodes
	for iteration := 0; iteration < 3; iteration++ { // Limit iterations
		var toQuery []identity.KademliaNode

		// Select ALPHA closest unqueried nodes
		for _, node := range candidates {
			if !queried[node.ID] && len(toQuery) < ALPHA {
				toQuery = append(toQuery, node)
			}
		}

		if len(toQuery) == 0 {
			break // No more nodes to query
		}

		// Query selected nodes in parallel
		type result struct {
			nodes []identity.KademliaNode
			err   error
		}

		resultChan := make(chan result, len(toQuery))

		for _, node := range toQuery {
			queried[node.ID] = true
			go func(n identity.KademliaNode) {
				msg := message.Message{
					Type:   message.FIND_NODE,
					ID:     fmt.Sprintf("iterative_find_node-%d", time.Now().UnixNano()),
					Target: target,
					// Shard:  shard_num,
					TargetShard: shard_num,
					SenderShard: kn.ShardNum,
				}

				response, err := kn.sendMessage(n, msg)
				if err != nil {
					resultChan <- result{nil, err}
					return
				}

				if response.Type == message.FIND_NODE_RESPONSE {
					resultChan <- result{response.Nodes, nil}
				} else {
					resultChan <- result{nil, fmt.Errorf("unexpected response type")}
				}
			}(node)
		}

		// Collect results
		newNodes := make(map[identity.KademliaNodeID]identity.KademliaNode)
		for i := 0; i < len(toQuery); i++ {
			select {
			case res := <-resultChan:
				if res.err == nil {
					for _, node := range res.nodes {
						newNodes[node.ID] = node
					}
				}
			case <-time.After(2 * time.Second):
				// Timeout
			}
		}

		// Add new nodes to candidates
		for _, node := range newNodes {
			candidates = append(candidates, node)
		}

		// Sort candidates by distance to target
		sort.Slice(candidates, func(i, j int) bool {
			distI := candidates[i].ID.XOR(target)
			distJ := candidates[j].ID.XOR(target)
			return distI.Cmp(distJ) < 0
		})

		// Keep only K closest candidates
		if len(candidates) > K {
			candidates = candidates[:K]
		}
	}

	// Return closest nodes found
	if len(candidates) > K {
		return candidates[:K], nil
	}
	return candidates, nil
}

// Broadcast sends a message to all connected nodes in the network
func (kn *Kademlia) Broadcast(msg message.Message) error {
	messageID := fmt.Sprintf("broadcast-%s-%d", kn.node.ID.String()[:8], time.Now().UnixNano())
	// Mark our own message as seen
	kn.messageMutex.Lock()
	kn.seenMessages[messageID] = true
	kn.messageMutex.Unlock()

	// fmt.Printf("[노드 %s:%d] 브로드캐스트 메시지 발송: '%s'\n",
	// 	kn.node.IP, kn.node.Port, msg.MessageData)

	return kn.sendBroadcastToAllNodes(msg)
}

// sendBroadcastToAllNodes sends broadcast message to all nodes in routing table
func (kn *Kademlia) sendBroadcastToAllNodes(msg message.Message) error {
	var allContacts []Contact

	// Collect all contacts from all buckets
	// for _, bucket := range kn.routingTable.buckets {
	// 	allContacts = append(allContacts, bucket.GetContacts()...)
	// }

	if _, exists := kn.routingTable[msg.TargetShard]; !exists {
		log.Infof("shard %v routing table doesn't exists routingTabel, initialize...", msg.TargetShard)
		kn.routingTable[msg.TargetShard] = NewRoutingTable(kn.node)
	}

	for _, bucket := range kn.routingTable[msg.TargetShard].buckets {
		allContacts = append(allContacts, bucket.GetContacts()...)
	}

	if len(allContacts) == 0 {
		return fmt.Errorf("no nodes in routing table to broadcast to")
	}

	successCount := 0
	for _, contact := range allContacts {
		// Don't send to ourselves
		if contact.Node.ID == kn.node.ID {
			continue
		}

		go func(node identity.KademliaNode) {
			_, err := kn.sendMessage(node, msg)
			// _, err := kn.sendMessage(node, msg)
			if err != nil {
				log.Errorf("[노드 %s:%d] 브로드캐스트 전송 실패 to %s:%d - %v\n",
					kn.node.IP, kn.node.Port, node.IP, node.Port, err)
			}
		}(contact.Node)

		successCount++
	}

	// fmt.Printf("[노드 %s:%d] 브로드캐스트 메시지를 %d개 노드에 전송 완료\n",
	// 	kn.node.IP, kn.node.Port, successCount)

	return nil
}

// forwardBroadcast forwards a broadcast message to other nodes
func (kn *Kademlia) forwardBroadcast(originalMsg message.Message) {
	// Decrease TTL
	forwardMsg := originalMsg
	forwardMsg.TTL = originalMsg.TTL - 1
	forwardMsg.ID = fmt.Sprintf("broadcast-forward-%d", time.Now().UnixNano())

	var allContacts []Contact

	// Collect all contacts from all buckets
	// for _, bucket := range kn.routingTable[originalMsg.Shard].buckets {
	// 	allContacts = append(allContacts, bucket.GetContacts()...)
	// }
	for _, bucket := range kn.routingTable[originalMsg.TargetShard].buckets {
		allContacts = append(allContacts, bucket.GetContacts()...)
	}

	forwardCount := 0
	for _, contact := range allContacts {
		// Don't send back to sender or ourselves
		if contact.Node.ID == originalMsg.Sender.ID || contact.Node.ID == kn.node.ID {
			continue
		}

		go func(node identity.KademliaNode) {
			kn.sendMessage(node, forwardMsg)
			// _, err := kn.sendMessage(node, forwardMsg)
			// if err != nil {
			// 	fmt.Printf("[노드 %s:%d] 브로드캐스트 포워딩 실패 to %s:%d - %v\n",
			// 		kn.node.IP, kn.node.Port, node.IP, node.Port, err)
			// }
		}(contact.Node)

		forwardCount++
	}

	// if forwardCount > 0 {
	// 	fmt.Printf("[노드 %s:%d] 브로드캐스트 메시지를 %d개 노드에 포워딩\n",
	// 		kn.node.IP, kn.node.Port, forwardCount)
	// }
}

// GetConnectedNodes returns all nodes in the routing table
func (kn *Kademlia) GetConnectedNodes(shard_num int) []identity.KademliaNode {
	var allNodes []identity.KademliaNode

	for _, bucket := range kn.routingTable[shard_num].buckets {
		contacts := bucket.GetContacts()
		for _, contact := range contacts {
			allNodes = append(allNodes, contact.Node)
		}
	}

	return allNodes
}

// GetNodeCount returns the number of connected nodes
func (kn *Kademlia) GetNodeCount(shard_num int) int {
	return len(kn.GetConnectedNodes(shard_num))
}

func (kn *Kademlia) putData(msg message.Message) {
	switch msg.DataType {
	case "consensus":
		// fmt.Printf("[노드 %s:%d] 브로드캐스트 합ti.IP, msg.Sender.Port)
		kn.ConsensusMessages <- msg
	case "shard":
		log.Infof("[%v:%v] received shard message: %v", kn.node.IP, kn.node.Port, msg)
		kn.ShardMessages <- msg
	}
}

func (kn *Kademlia) BroadcastToShard(msg message.ShardMessage) {
	log.Infof("Sending Message to %v shard, Message: %v", msg.TargetShard, msg.Message)

	if err := kn.Join(msg.TargetShard); err != nil {
		log.Errorf("[%v:%v] error while joining to target shard %v, err: %v ", kn.node.IP, kn.node.Port, msg.TargetShard, err)
		return
	}

	broadcast_msg := message.Message{
		ID:        fmt.Sprintf("broadcast-%s-%d", kn.node.ID.String()[:8], time.Now().UnixNano()),
		Type:      message.BROADCAST,
		TTL:       5,
		Timestamp: time.Now(),
		DataType:  "shard",
		Data:      msg,
		// Shard:     msg.TargetShard,
		TargetShard: msg.TargetShard,
		SenderShard: kn.ShardNum,
	}
	kn.Broadcast(broadcast_msg)

	kn.Leave(msg.TargetShard)
}

// Leave gracefully leaves the Kademli	a network
func (kn *Kademlia) Leave(shard_num int) error {
	// log.Infof("Node %s:%d is leaving the shard network %v", kn.node.IP, kn.node.Port, shard_num)

	// Notify all connected nodes that we are leaving
	leaveMsg := message.Message{
		Type:      message.LEAVE,
		ID:        fmt.Sprintf("leave-%d", time.Now().UnixNano()),
		Sender:    kn.node,
		Timestamp: time.Now(),
		// Shard:     shard_num,
		TargetShard: shard_num,
		SenderShard: kn.ShardNum,
	}

	// Get all connected nodes
	allNodes := kn.GetConnectedNodes(shard_num)

	// Send leave notification to all nodes
	successCount := 0
	for _, node := range allNodes {
		if node.ID == kn.node.ID {
			continue // Skip ourselves
		}

		go func(targetNode identity.KademliaNode) {
			response, err := kn.sendMessage(targetNode, leaveMsg)
			if err != nil {
				log.Debugf("Failed to send leave message to %s:%d - %v",
					targetNode.IP, targetNode.Port, err)
				return
			}

			if response.Type == message.LEAVE_ACK {
				// log.Debugf("Received leave acknowledgment from %s:%d",targetNode.IP, targetNode.Port)
			}
		}(node)
		successCount++
	}

	// Wait a bit for leave messages to be sent
	time.Sleep(100 * time.Millisecond)

	// Clear our routing table
	kn.routingTable[shard_num] = NewRoutingTable(kn.node)

	// Clear local storage (optional - depends on requirements)
	// kn.mutex.Lock()
	// kn.storage = make(map[string]string)
	// kn.mutex.Unlock()

	// Clear seen messages
	// kn.messageMutex.Lock()
	// kn.seenMessages = make(map[string]bool)
	// kn.messageMutex.Unlock()

	// Stop the node
	// kn.Stop()

	// log.Infof("Node %s:%d has left the network (notified %d nodes)",
	// 	kn.node.IP, kn.node.Port, successCount)

	return nil
}
