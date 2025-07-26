package shard

import "Pororo-droid/go-byshard/message"

type Shard interface {
	Handle(interface{})
	Propose(message.ShardRequest)
	SetToPrimary()
	GetForward() chan message.Request
	HandleConsensusResult(message.ConsenusResult)
	GetBroadcastMessages() chan message.ShardMessage
}
