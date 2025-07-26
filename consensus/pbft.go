package consensus

import (
	"Pororo-droid/go-byshard/config"
	"Pororo-droid/go-byshard/crypto"
	"Pororo-droid/go-byshard/db"
	"Pororo-droid/go-byshard/log"
	"Pororo-droid/go-byshard/message"
	"Pororo-droid/go-byshard/types"
	"Pororo-droid/go-byshard/utils"
	"crypto/ecdsa"
	"crypto/elliptic"
	"fmt"
	"sync"
	"time"
)

const (
	REQUEST = iota
	PREPREPARE
	PREPARE
	COMMIT
	REPLY
)

type PBFT struct {
	id   string
	ip   string
	port int

	View     types.View
	Sequence types.Sequence

	CommitteeNum int
	isPrimary    bool

	privateKey *ecdsa.PrivateKey

	mu                  *sync.Mutex
	messageID           int
	BroadcastMessages   chan message.Message
	receivedPreprepares map[string]*message.Preprepare
	receivedPrepares    map[string]map[string]*message.Prepare
	receivedCommits     map[string]map[string]*message.Commit
	// receivedPrepares   map[string][]*message.Prepare
	// receivedCommits    map[string]*message.Commit

	stateDB db.DB

	forward chan message.ConsenusResult
}

func NewPBFT(ip string, port int, private_key *ecdsa.PrivateKey, stateDB db.DB) *PBFT {
	return &PBFT{
		id:                  fmt.Sprintf("%s-%d", ip, port),
		ip:                  ip,
		port:                port,
		View:                0,
		Sequence:            0,
		CommitteeNum:        4,
		isPrimary:           false,
		privateKey:          private_key,
		BroadcastMessages:   make(chan message.Message, config.GetConfig().ChanelSize),
		receivedPreprepares: make(map[string]*message.Preprepare),
		receivedPrepares:    make(map[string]map[string]*message.Prepare),
		receivedCommits:     make(map[string]map[string]*message.Commit),
		mu:                  new(sync.Mutex),
		stateDB:             stateDB,
		forward:             make(chan message.ConsenusResult, config.GetConfig().ChanelSize),
	}
}

func (n *PBFT) SetToPrimary() {
	n.isPrimary = true
}

func (n *PBFT) Handle(msg interface{}) {
	map_data, ok := msg.(map[string]interface{})
	if !ok {
		fmt.Println("형변환 시도 중 에러 발생", msg)
		return
	}

	var request_msg message.Request
	var preprepare_msg message.Preprepare
	var prepare_msg message.Prepare
	var commit_msg message.Commit

	if err := utils.MapToStruct(map_data, &request_msg); err == nil {
		n.Propose(request_msg)
		return
	}

	if err := utils.MapToStruct(map_data, &preprepare_msg); err == nil {
		n.HandlePreprepare(preprepare_msg)
		return
	}

	if err := utils.MapToStruct(map_data, &prepare_msg); err == nil {
		n.HandlePrepare(prepare_msg)
		return
	}

	if err := utils.MapToStruct(map_data, &commit_msg); err == nil {
		n.HandleCommit(commit_msg)
		return
	}

	log.Infof("[%s] Unavailabe message\n", n.id)
}

// ProcessRequest processes a client request (Primary only)
func (n *PBFT) Propose(req message.Request) {
	if !n.isPrimary {
		return
	}

	n.mu.Lock()
	defer n.mu.Unlock()

	n.Sequence++
	digest, err := crypto.ECDSASign(n.privateKey, utils.ToByte(req))
	if err != nil {
		fmt.Errorf("[%v] Error while signing ECDSA %v", n.id, err)
	}

	preprepare_msg := message.Preprepare{
		Type:     "PREPREPARE",
		View:     n.View,
		Sequence: n.Sequence,
		// PublicKey: n.privateKey.PublicKey,
		PublicKey_X: n.privateKey.PublicKey.X.Bytes(),
		PublicKey_Y: n.privateKey.PublicKey.Y.Bytes(),
		Request:     req,
		Digest:      digest,
		Timestamp:   time.Now(),
	}

	// Broadcast to all backup nodes
	n.Broadcast(preprepare_msg)

	log.Infof("[%s] Sent Pre-prepare for sequence %d\n", n.id, n.Sequence)
	go n.HandlePreprepare(preprepare_msg)
}

func (n *PBFT) HandlePreprepare(msg message.Preprepare) {
	n.mu.Lock()
	defer n.mu.Unlock()

	key := fmt.Sprintf("%d-%d", msg.View, msg.Sequence)
	if _, exists := n.receivedPreprepares[key]; exists {
		log.Infof("[%s] 같은 view-seq에 대해 pre-prepare 수신, msg: %v", n.id, msg)
		return
	}

	// 메시지 검증
	if msg.View != n.View {
		log.Infof("[HandelPreprepare][%s] View가 일치하지 않음 (현재: %d, 메시지: %d)", n.id, n.View, msg.View)
		return
	}

	sender_pubKey := crypto.UncompressedBytesToPublicKey(elliptic.P256(), msg.PublicKey_X, msg.PublicKey_Y)

	result, err := crypto.ECDSAVerify(*sender_pubKey, utils.ToByte(msg.Request), msg.Digest)
	if err != nil {
		log.Infof("[HandelPreprepare][%s] 메시지 검증 중 에러 발생\n", n.id)
		return
	}

	if !result {
		log.Infof("[HandelPreprepare][%s] 메시지 서명이 일치하지 않음\n", n.id)
		return
	}

	log.Infof("[HandelPreprepare][%s] preprepare 메시지 수신 및 검증 완료 - View: %d, Seq: %d\n",
		n.id, msg.View, msg.Sequence)

	n.receivedPreprepares[key] = &msg

	digest, err := crypto.ECDSASign(n.privateKey, utils.ToByte(msg))

	if err != nil {
		log.Errorf("[%s] Error while signing ECDSA %s", n.id, err)
		return
	}

	prepare_msg := message.Prepare{
		Type:        "PREPARE",
		View:        msg.View,
		Sequence:    msg.Sequence,
		PublicKey_X: n.privateKey.PublicKey.X.Bytes(),
		PublicKey_Y: n.privateKey.PublicKey.Y.Bytes(),
		Preprepare:  msg,
		Digest:      digest,
		Timestamp:   time.Now(),
		ID:          n.id,
	}

	go n.Broadcast(prepare_msg)
	log.Infof("[HandelPreprepare][%s] Broadcast Prepare for sequence %d\n", n.id, msg.Sequence)

	go n.HandlePrepare(prepare_msg)
}

func (n *PBFT) HandlePrepare(msg message.Prepare) {
	n.mu.Lock()
	defer n.mu.Unlock()

	log.Infof("[HandelPrepare][%s] Received Prepare message from %s\n", n.id, msg.ID)

	key := fmt.Sprintf("%d-%d", msg.View, msg.Sequence)

	// 메시지 검증
	if msg.View != n.View {
		log.Infof("[%s] View가 일치하지 않음 (현재: %d, 메시지: %d)", n.id, n.View, msg.View)
		return
	}

	sender_pubKey := crypto.UncompressedBytesToPublicKey(elliptic.P256(), msg.PublicKey_X, msg.PublicKey_Y)
	result, err := crypto.ECDSAVerify(*sender_pubKey, utils.ToByte(msg.Preprepare), msg.Digest)
	if err != nil {
		log.Infof("[HandlePrepare][%s] 메시지 검증 중 에러 발생\n", n.id)
		return
	}

	if !result {
		log.Infof("[Preprare][%s] 메시지 서명이 일치하지 않음\n", n.id)
		return
	}

	if _, exists := n.receivedPrepares[key][msg.ID]; exists {
		log.Infof("[HandlePrepare][%s] %s 노드로부터 중복 메시지 수신", n.id, msg.ID)
		return
	}

	// prepare 메시지 저장
	if n.receivedPrepares[key] == nil {
		n.receivedPrepares[key] = make(map[string]*message.Prepare, 0)
	}
	// n.receivedPrepares[key] = append(n.receivedPrepares[key], &msg)
	n.receivedPrepares[key][msg.ID] = &msg

	log.Infof("[HandlePrepare][%s] prepare 메시지 수신 from %v- View: %d, Seq: %d (총 %d개)\n",
		n.id, msg.ID, msg.View, msg.Sequence, len(n.receivedPrepares[key]))

	f := (n.CommitteeNum - 1) / 3 // 최대 Byzantine 노드 수
	requiredPrepares := 2*f + 1

	if len(n.receivedPrepares[key]) != requiredPrepares {
		return
	}

	log.Infof("[HandlePrepare][%s] Prepare phase 완료! (%d/%d 준비 메시지 수집)\n",
		n.id, len(n.receivedPrepares[key]), requiredPrepares)

	var prepare_list []*message.Prepare
	for _, prepare := range n.receivedPrepares[key] {
		prepare_list = append(prepare_list, prepare)
	}

	digest, err := crypto.ECDSASign(n.privateKey, utils.ToByte(prepare_list))

	if err != nil {
		log.Errorf("[%s] Error while signing ECDSA %s", n.id, err)
		return
	}

	prepare_msg := message.Commit{
		Type:        "COMMIT",
		View:        msg.View,
		Sequence:    msg.Sequence,
		PublicKey_X: n.privateKey.PublicKey.X.Bytes(),
		PublicKey_Y: n.privateKey.PublicKey.Y.Bytes(),
		PrepareList: prepare_list,
		Digest:      digest,
		Timestamp:   time.Now(),
		ID:          n.id,
	}

	go n.Broadcast(prepare_msg)
	log.Infof("[HandlePrepare][%s] Broadcast Prepare for sequence %d\n", n.id, msg.Sequence)

	go n.HandleCommit(prepare_msg)
}

func (n *PBFT) HandleCommit(msg message.Commit) {
	n.mu.Lock()
	defer n.mu.Unlock()

	log.Infof("[HandelCommit][%s] Received Commit message from %s\n", n.id, msg.ID)

	key := fmt.Sprintf("%d-%d", msg.View, msg.Sequence)

	// 메시지 검증
	if msg.View != n.View {
		log.Infof("[%s] View가 일치하지 않음 (현재: %d, 메시지: %d)", n.id, n.View, msg.View)
		return
	}

	sender_pubKey := crypto.UncompressedBytesToPublicKey(elliptic.P256(), msg.PublicKey_X, msg.PublicKey_Y)
	result, err := crypto.ECDSAVerify(*sender_pubKey, utils.ToByte(msg.PrepareList), msg.Digest)
	if err != nil {
		log.Infof("[HandleCommit][%s] 메시지 검증 중 에러 발생\n", n.id)
		return
	}

	if !result {
		log.Infof("[HandleCommit][%s] 메시지 서명이 일치하지 않음\n", n.id)
		return
	}

	// PrepareList 서명 검증

	// Prepares 검사
	// if commits, exists := n.receivedCommits[key]; exists {
	// 	for _, commit := range commits {
	// 		commit_pubKey := crypto.UncompressedBytesToPublicKey(elliptic.P256(), commit.PublicKey_X, commit.PublicKey_Y)
	// 		if commit_pubKey == sender_pubKey {
	// 			log.Infof("[HandleCommit][%s] %s 노드로부터 중복 메시지 수신", n.id, sender_pubKey)
	// 			return
	// 		}
	// 	}
	// }

	if _, exists := n.receivedCommits[key][msg.ID]; exists {
		log.Infof("[HandleCommit][%s] %s 노드로부터 중복 메시지 수신", n.id, msg.ID)
		return
	}

	// Commit 메시지 저장
	if n.receivedCommits[key] == nil {
		n.receivedCommits[key] = make(map[string]*message.Commit, 0)
	}
	// n.receivedCommits[key] = append(n.receivedCommits[key], &msg)
	n.receivedCommits[key][msg.ID] = &msg

	f := (n.CommitteeNum - 1) / 3 // 최대 Byzantine 노드 수
	requiredCommits := 2*f + 1

	log.Infof("[HandleCommit][%s] Commit 메시지 수신 From %s - View: %d, Seq: %d (총 %d개)\n",
		n.id, msg.ID, msg.View, msg.Sequence, len(n.receivedCommits[key]))

	if len(n.receivedCommits[key]) != requiredCommits {
		return
	}

	log.Infof("[HandleCommit][%s] Commit 완료\n", n.id)

	// Execute
	n.execute(key)
}

func (n *PBFT) Broadcast(msg interface{}) {
	network_msg := message.Message{
		Type:      message.BROADCAST,
		MessageID: fmt.Sprintf("%s:%d:%d", n.id, n.port, n.messageID),
		TTL:       5,
		Timestamp: time.Now(),
		DataType:  "consensus",
		Data:      msg,
	}
	n.messageID += 1

	n.BroadcastMessages <- network_msg
}

func (n *PBFT) GetBroadcastMessages() chan message.Message {
	return n.BroadcastMessages
}

// execute를 언제할지, 그리고 abort시 어떤 정보들을 보내야하는지 고안 필요
func (n *PBFT) execute(key string) {
	preprepare := n.receivedPreprepares[key]
	request := preprepare.Request

	var result message.ConsenusResult
	result.Request = request

	if _, exists := n.stateDB[request.Target]; !exists {
		n.stateDB[request.Target] = 0
	}

	// Condition is more than.... need to fix
	// 우선은 무조건 통과하게 만듦.., (commit-vote 구현 중....)
	// if n.stateDB[request.Target] > request.Condition {
	// 	// ABORT
	// 	result.Result = "abort-vote"
	// 	n.forward <- result
	// 	return
	// }

	switch request.Operation {
	case "Add":
		n.stateDB[request.Target] += request.Amount
	case "Sub":
		n.stateDB[request.Target] -= request.Amount
	}

	// Continue
	log.Infof("[Execute][%v] Commit-Vote", n.id)
	result.Result = "commit-vote"
	n.forward <- result
}

func (n *PBFT) GetConsensusResults() chan message.ConsenusResult {
	return n.forward
}
