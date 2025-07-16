package consensus

import (
	"Pororo-droid/go-byshard/crypto"
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

	View     types.View
	Sequence types.Sequence

	privateKey *ecdsa.PrivateKey

	mu                 sync.Mutex
	BroadcastMessages  chan message.Message
	receivedPrepreares map[string]*message.Preprepare
}

func NewPBFT(ip string, port int, private_key *ecdsa.PrivateKey) *PBFT {
	return &PBFT{
		id:                 fmt.Sprintf("%s-%d", ip, port),
		ip:                 ip,
		port:               port,
		View:               0,
		Sequence:           0,
		privateKey:         private_key,
		BroadcastMessages:  make(chan message.Message, 100),
		receivedPrepreares: make(map[string]*message.Preprepare),
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

	if err := mapToStruct(map_data, &request_msg); err == nil {
		n.Propose(request_msg)
		return
	}

	if err := mapToStruct(map_data, &preprepare_msg); err == nil {
		n.Prepare(preprepare_msg)
		return
	}

	fmt.Printf("[%s] Unavailabe message\n", n.id)
}

// ProcessRequest processes a client request (Primary only)
func (n *PBFT) Propose(req message.Request) {
	fmt.Println(req)
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

	fmt.Printf("Primary Node Sent Pre-prepare for sequence %d\n", n.Sequence)
	n.Prepare(preprepare_msg)
}

func (n *PBFT) Prepare(msg message.Preprepare) {
	key := fmt.Sprintf("%d-%d", msg.View, msg.Sequence)
	if _, exists := n.receivedPrepreares[key]; exists {
		fmt.Printf("[%s] 같은 view-seq에 대해 pre-prepare 수신", n.id)
		return
	}

	// 메시지 검증
	if msg.View != n.View {
		fmt.Printf("[%s] View가 일치하지 않음 (현재: %d, 메시지: %d)", n.id, n.View, msg.View)
		return
	}

	sender_pubKey := crypto.UncompressedBytesToPublicKey(elliptic.P256(), msg.PublicKey_X, msg.PublicKey_Y)

	result, err := crypto.ECDSAVerify(*sender_pubKey, utils.ToByte(msg.Request), msg.Digest)
	if err != nil {
		fmt.Printf("[Prepare][%s] 메시지 검증 중 에러 발생\n", n.id)
		return
	}

	if !result {
		fmt.Printf("[Preprare][%s] 메시지 서명이 일치하지 않음\n", n.id)
		return
	}

	fmt.Printf("[Prepare][%s] preprepare 메시지 수신 및 검증 완료 - View: %d, Seq: %d\n",
		n.id, msg.View, msg.Sequence)

	n.receivedPrepreares[key] = &msg

	digest, err := crypto.ECDSASign(n.privateKey, utils.ToByte(msg))
	fmt.Printf("[%s] 1111111\n", n.id)
	if err != nil {
		fmt.Errorf("[%v] Error while signing ECDSA %v", n.id, err)
		fmt.Printf("[%s] Error while signing ECDSA %s", n.id, err)
		return
	}
	fmt.Printf("[%s] 2222222\n", n.id)
	prepare_msg := message.Prepare{
		Type:        "PREPARE",
		View:        n.View,
		Sequence:    n.Sequence,
		PublicKey_X: n.privateKey.PublicKey.X.Bytes(),
		PublicKey_Y: n.privateKey.PublicKey.Y.Bytes(),
		Preprepare:  msg,
		Digest:      digest,
		Timestamp:   time.Now(),
	}
	n.Broadcast(prepare_msg)
	fmt.Printf("[%s] 333333333\n", n.id)
	fmt.Printf("[%s] Sent Prepare for sequence %d\n", n.id, n.Sequence)
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
		TTL:       5,
		Timestamp: time.Now(),
		DataType:  "consensus",
		Data:      msg,
	}

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

func deepCopyMap(original map[string]interface{}) map[string]interface{} {
	copied := make(map[string]interface{})
	for key, value := range original {
		copied[key] = value
	}
	return copied
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
