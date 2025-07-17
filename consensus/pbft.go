package consensus

import (
	"Pororo-droid/go-byshard/config"
	"Pororo-droid/go-byshard/crypto"
	"Pororo-droid/go-byshard/log"
	"Pororo-droid/go-byshard/message"
	"Pororo-droid/go-byshard/types"
	"Pororo-droid/go-byshard/utils"
	"crypto/ecdsa"
	"crypto/elliptic"
	"encoding/json"
	"fmt"
	"reflect"
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

	View         types.View
	Sequence     types.Sequence
	CommitteeNum int

	privateKey *ecdsa.PrivateKey

	mu                 sync.Mutex
	messageID          int
	BroadcastMessages  chan message.Message
	receivedPrepreares map[string]*message.Preprepare
	receivedPrepares   map[string][]*message.Prepare
	receivedCommits    map[string][]*message.Commit
}

func NewPBFT(ip string, port int, private_key *ecdsa.PrivateKey) *PBFT {
	return &PBFT{
		id:                 fmt.Sprintf("%s-%d", ip, port),
		ip:                 ip,
		port:               port,
		View:               0,
		Sequence:           0,
		CommitteeNum:       4,
		privateKey:         private_key,
		BroadcastMessages:  make(chan message.Message, config.GetConfig().ChanelSize),
		receivedPrepreares: make(map[string]*message.Preprepare),
		receivedPrepares:   make(map[string][]*message.Prepare),
		receivedCommits:    make(map[string][]*message.Commit),
	}
}

func (n *PBFT) Handle(msg interface{}) {
	map_data, ok := msg.(map[string]interface{})
	if !ok {
		fmt.Println("형변환 시도 중 에러 발생")
		return
	}

	var request_msg message.Request
	var preprepare_msg message.Preprepare
	var prepare_msg message.Prepare
	var commit_msg message.Commit

	if err := mapToStruct(map_data, &request_msg); err == nil {
		n.Propose(request_msg)
		return
	}

	if err := mapToStruct(map_data, &preprepare_msg); err == nil {
		n.HandlePreprepare(preprepare_msg)
		return
	}

	if err := mapToStruct(map_data, &prepare_msg); err == nil {
		n.HandlePrepare(prepare_msg)
		return
	}

	if err := mapToStruct(map_data, &commit_msg); err == nil {
		n.HandleCommit(commit_msg)
		return
	}

	log.Infof("[%s] Unavailabe message\n", n.id)
}

// ProcessRequest processes a client request (Primary only)
func (n *PBFT) Propose(req message.Request) {
	if !n.isPrimary() {
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

	// log.Infof("[%s] Sent Pre-prepare for sequence %d\n", n.id, n.Sequence)
	n.HandlePreprepare(preprepare_msg)
}

func (n *PBFT) HandlePreprepare(msg message.Preprepare) {
	key := fmt.Sprintf("%d-%d", msg.View, msg.Sequence)
	if _, exists := n.receivedPrepreares[key]; exists {
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

	n.receivedPrepreares[key] = &msg

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
	}

	n.Broadcast(prepare_msg)
	log.Infof("[HandelPreprepare][%s] Broadcast Prepare for sequence %d\n", n.id, msg.Sequence)

	n.HandlePrepare(prepare_msg)
}

func (n *PBFT) HandlePrepare(msg message.Prepare) {
	log.Infof("[HandelPrepare][%s] Received Prepare message\n", n.id)

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

	if prepares, exists := n.receivedPrepares[key]; exists {
		for _, prepare := range prepares {
			prepare_pubKey := crypto.UncompressedBytesToPublicKey(elliptic.P256(), prepare.PublicKey_X, prepare.PublicKey_Y)
			if prepare_pubKey == sender_pubKey {
				log.Infof("[HandlePrepare][%s] %s 노드로부터 중복 메시지 수신", n.id, sender_pubKey)
				return
			}
		}
	}

	// prepare 메시지 저장
	if n.receivedPrepares[key] == nil {
		n.receivedPrepares[key] = make([]*message.Prepare, 0)
	}
	n.receivedPrepares[key] = append(n.receivedPrepares[key], &msg)

	log.Infof("[HandlePrepare][%s] prepare 메시지 수신 - View: %d, Seq: %d (총 %d개)\n",
		n.id, msg.View, msg.Sequence, len(n.receivedPrepares[key]))

	f := (n.CommitteeNum - 1) / 3 // 최대 Byzantine 노드 수
	requiredPrepares := 2*f + 1

	if len(n.receivedPrepares[key]) != requiredPrepares {
		return
	}

	log.Infof("[HandlePrepare][%s] Prepare phase 완료! (%d/%d 준비 메시지 수집)\n",
		n.id, len(n.receivedPrepares[key]), requiredPrepares)

	prepare_list := n.receivedPrepares[key]
	digest, err := crypto.ECDSASign(n.privateKey, utils.ToByte(prepare_list))

	if err != nil {
		fmt.Errorf("[%v] Error while signing ECDSA %v", n.id, err)
		log.Infof("[%s] Error while signing ECDSA %s", n.id, err)
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
	}

	n.Broadcast(prepare_msg)
	log.Infof("[HandlePrepare][%s] Broadcast Prepare for sequence %d\n", n.id, msg.Sequence)

	n.HandleCommit(prepare_msg)
}

func (n *PBFT) HandleCommit(msg message.Commit) {
	log.Infof("[HandelCommit][%s] Received Commit message\n", n.id)

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
	log.Infof("[HandleCommit][%s] Commit 완료\n", n.id)
	// Prepares 검사
	if commits, exists := n.receivedCommits[key]; exists {
		for _, commit := range commits {
			commit_pubKey := crypto.UncompressedBytesToPublicKey(elliptic.P256(), commit.PublicKey_X, commit.PublicKey_Y)
			if commit_pubKey == sender_pubKey {
				log.Infof("[HandleCommit][%s] %s 노드로부터 중복 메시지 수신", n.id, sender_pubKey)
				return
			}
		}
	}

	// Commit 메시지 저장
	if n.receivedCommits[key] == nil {
		n.receivedCommits[key] = make([]*message.Commit, 0)
	}
	n.receivedCommits[key] = append(n.receivedCommits[key], &msg)

	f := (n.CommitteeNum - 1) / 3 // 최대 Byzantine 노드 수
	requiredCommits := 2*f + 1

	log.Infof("[HandleCommit][%s] Commit 메시지 수신 - View: %d, Seq: %d (총 %d개)\n",
		n.id, msg.View, msg.Sequence, len(n.receivedCommits[key]))

	if len(n.receivedCommits[key]) != requiredCommits {
		return
	}

	log.Infof("[HandleCommit][%s] Commit 완료\n", n.id)

	// Execute
}

func (n *PBFT) isPrimary() bool {
	if n.port == 8002 {
		return true
	}
	return false
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

func mapToStruct(data map[string]interface{}, result interface{}) error {
	if err := checkStruct(data, result); err != nil {
		return err
	}
	// map을 JSON으로 변환
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	// JSON을 struct로 변환
	if err := json.Unmarshal(jsonData, result); err != nil {
		return err
	}

	t := reflect.TypeOf(result)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	// 모든 필드가 채워졌는지 검증
	// return validateAllFieldsFilled(result)
	return nil
}

func checkStruct(a map[string]interface{}, b interface{}) error {
	t := reflect.TypeOf(b)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	for i := 0; i < t.NumField(); i++ {
		var is_included bool = false
		for k, _ := range a {
			if k == t.Field(i).Name {
				is_included = true
			}
		}

		if !is_included {
			return fmt.Errorf("구조체가 일치하지 않습니다.")
		}
	}

	return nil
}
