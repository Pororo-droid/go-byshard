package node

import (
	"Pororo-droid/go-byshard/consensus"
	"Pororo-droid/go-byshard/network"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
)

type Node struct {
	privateKey *ecdsa.PrivateKey
	network    *network.Kademlia
	Consensus  consensus.Consensus
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

	switch alg {
	case "pbft":
		node.Consensus = consensus.NewPBFT(ip, port, privateKey)
	}

	return *node
}

func (n *Node) Run() {
	for {
		select {
		case consensus_msg := <-n.network.ConsensusMessages:
			n.Consensus.Handle(consensus_msg.Data)
		case broadcast_msg := <-n.Consensus.GetBroadcastMessages():
			n.network.Broadcast(broadcast_msg)
		}
	}
}
