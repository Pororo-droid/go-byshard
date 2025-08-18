package node

import (
	"Pororo-droid/go-byshard/consensus"
	"Pororo-droid/go-byshard/db"
	"Pororo-droid/go-byshard/network"
	"Pororo-droid/go-byshard/shard"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
)

type Node struct {
	privateKey *ecdsa.PrivateKey
	network    *network.Kademlia
	Consensus  consensus.Consensus
	Shard      shard.Shard

	stateDB db.DB
}

func NewNode(ip string, port int, alg string, shard_num int) Node {
	node := new(Node)

	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		panic(err)
	}

	network := network.NewKademlia(ip, port, shard_num)
	network.Setup()

	node.privateKey = privateKey
	node.network = network
	node.stateDB = db.NewDB()

	switch alg {
	case "pbft":
		node.Consensus = consensus.NewPBFT(ip, port, privateKey, node.stateDB)
	}

	node.Shard = shard.NewLinear(ip, port)

	return *node
}

func (n *Node) Run() {
	for {
		select {
		case consensus_msg := <-n.network.ConsensusMessages:
			n.Consensus.Handle(consensus_msg.Data)
		case shard_msg := <-n.network.ShardMessages:
			n.Shard.Handle(shard_msg)
		case forward_msg := <-n.Shard.GetForward():
			n.Consensus.Propose(forward_msg)
		case forward_msg := <-n.Consensus.GetConsensusResults():
			n.Shard.HandleConsensusResult(forward_msg)
		case broadcast_msg := <-n.Consensus.GetBroadcastMessages():
			broadcast_msg.Sender = n.network.GetNodeInfo()
			broadcast_msg.Shard = n.network.ShardNum
			n.network.Broadcast(broadcast_msg)
		case broadcast_msg := <-n.Shard.GetBroadcastMessages():
			n.network.BroadcastToShard(broadcast_msg)
		}
	}
}
