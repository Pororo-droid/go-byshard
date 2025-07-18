package consensus

import "Pororo-droid/go-byshard/message"

type Consensus interface {
	Propose(message.Request)
	SetToPrimary()
	Handle(interface{})
	GetBroadcastMessages() chan message.Message
}
