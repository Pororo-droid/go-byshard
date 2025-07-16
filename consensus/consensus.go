package consensus

import "Pororo-droid/go-byshard/message"

type Consensus interface {
	Propose(message.Request)
	Handle(interface{})
	GetBroadcastMessages() chan message.Message
}
