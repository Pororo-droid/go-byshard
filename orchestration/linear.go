package orchestration

import (
	"Pororo-droid/go-byshard/config"
	"Pororo-droid/go-byshard/log"
	"Pororo-droid/go-byshard/message"
	"Pororo-droid/go-byshard/utils"
	"fmt"
	"sync"
)

type Linear struct {
	id   string
	ip   string
	port int

	isPrimary bool
	messageID int

	BroadcastMessages chan message.ShardMessage
	forward           chan message.Request
	messages          map[string]message.ShardRequest
	// messages []message.ShardRequest

	mu *sync.Mutex

	shardNum int
}

func NewLinear(ip string, port int) *Linear {
	return &Linear{
		id:                fmt.Sprintf("%s-%d", ip, port),
		ip:                ip,
		port:              port,
		isPrimary:         false,
		messageID:         0,
		BroadcastMessages: make(chan message.ShardMessage, config.GetConfig().ChanelSize),
		forward:           make(chan message.Request, config.GetConfig().ChanelSize),
		messages:          make(map[string]message.ShardRequest),
		mu:                new(sync.Mutex),
	}
}

func (n *Linear) Handle(msg interface{}) {
	map_data, ok := msg.(map[string]interface{})
	if !ok {
		log.Errorf("형변환 시도 중 에러 발생")
		return
	}

	var shard_msg message.ShardMessage

	// 외부에서 들어온 Shard Message
	if err := utils.MapToStruct(map_data, &shard_msg); err == nil {
		log.Info(shard_msg)
		// n.HandleVote(vote_msg)
		var shard_request message.ShardRequest
		shard_request.Votes = append(shard_request.Votes, shard_msg.Message)

		n.Propose(shard_request)
		return
	}
}

func (n *Linear) HandleConsensusResult(result message.ConsenusResult) {
	// (Todo) handleCommitVote에서 에러가 발생하기 때문에 우선은 해당으로 락... 나중에 풀지 안풀지 결정해야 함
	if !n.isPrimary {
		return
	}

	shard_msg := n.messages[result.Request.GetID()]

	switch result.Result {
	case "commit-vote":
		n.handleCommitVote(shard_msg)
	case "abort-vote":
		n.handleAbort(shard_msg)
	}
}

func (n *Linear) handleCommitVote(shard_msg message.ShardRequest) {
	// (Todo) 리더만 저장하기 때문에 여기서 에러가 발생!
	shard_msg.Votes = shard_msg.Votes[1:]

	if len(shard_msg.Votes) == 0 {
		// Send to shards in commit
		for _, msg := range shard_msg.Commits {
			// broadcast msg to shards
			n.broadcastToShard(msg, utils.FindShard(msg.Target))
		}
		return
	}

	// broadcast msg to votes[0]
	// (Todo) 우선은 Vote[0]으로, interface로 변경할 필요가 있을까?
	n.broadcastToShard(shard_msg.Votes[0], utils.FindShard(shard_msg.Votes[0].Target))
}

func (n *Linear) handleAbort(shard_msg message.ShardRequest) {

}

func (n *Linear) Propose(msg message.ShardRequest) {
	if !n.isPrimary {
		return
	}

	n.messages[msg.Votes[0].GetID()] = msg
	n.forward <- msg.Votes[0]
}

func (n *Linear) SetToPrimary() {
	n.isPrimary = true
}

func (n *Linear) GetForward() chan message.Request {
	return n.forward
}

func (n *Linear) broadcastToShard(msg message.Request, shard int) {
	network_msg := message.ShardMessage{
		TargetShard: shard,
		Message:     msg,
	}

	n.BroadcastMessages <- network_msg
}

func (n *Linear) GetBroadcastMessages() chan message.ShardMessage {
	return n.BroadcastMessages
}
