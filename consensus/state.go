package consensus

import (
	"encoding/json"
	"fmt"
	"github.com/didchain/PBFT/message"
	"github.com/didchain/PBFT/p2pnetwork"
	"time"
)

type Consensus interface {
	StartConsensus()
	PrePrepare()
	Prepare()
	Commit()
}
type Stage int

const (
	Idle Stage = iota
	PrePrepared
	Prepared
	Committed
)

func (s Stage) String() string {
	switch s {
	case Idle:
		return "Idle"
	case PrePrepared:
		return "PrePrepared"
	case Prepared:
		return "Prepared"
	case Committed:
		return "Committed"
	}
	return "Unknown"
}

const StateTimerOut = 5 * time.Second
const MaxStateMsgNO = 100
const CheckPointInterval = 1 << 5          //32
const CheckPointK = 2 * CheckPointInterval //64

type RequestTimer struct {
	*time.Ticker
	IsOk bool
}

func newRequestTimer() *RequestTimer {
	tick := time.NewTicker(StateTimerOut)
	tick.Stop()
	return &RequestTimer{
		Ticker: tick,
		IsOk:   false,
	}
}

func (rt *RequestTimer) isRunning() bool {
	return rt.IsOk
}

func (rt *RequestTimer) tick() {
	if rt.IsOk {
		return
	}
	rt.Reset(StateTimerOut)
	rt.IsOk = true
}

func (rt *RequestTimer) tack() {
	rt.IsOk = false
	rt.Stop()
}

type EngineStatus int8

const (
	Syncing EngineStatus = iota
	Serving
	ViewChanging
)

func (es EngineStatus) String() string {
	switch es {
	case Syncing:
		return "Syncing block chain......"
	case Serving:
		return "Server consensus......"
	case ViewChanging:
		return "Changing views......"
	}

	return "Unknown"
}

type StateEngine struct {
	NodeID      int64 `json:"nodeID"`
	CurViewID   int64 `json:"viewID"`
	CurSequence int64 `json:"curSeq"`
	LasExeSeq   int64 `json:"lastExeSeq"`
	PrimaryID   int64 `json:"primaryID"`
	nodeStatus  EngineStatus

	Timer           *RequestTimer
	p2pWire         p2pnetwork.P2pNetwork
	MsgChan         <-chan *message.ConMessage
	nodeChan        chan<- *message.RequestRecord
	directReplyChan chan<- *message.Reply

	MiniSeq   int64 `json:"miniSeq"`
	MaxSeq    int64 `json:"maxSeq"`
	msgLogs   map[int64]*NormalLog
	checks    map[int64]*CheckPoint
	lastCP    *CheckPoint
	cliRecord map[string]*ClientRecord
	sCache    *VCCache
}

func InitConsensus(id int64, cChan chan<- *message.RequestRecord, rChan chan<- *message.Reply) *StateEngine {
	ch := make(chan *message.ConMessage, MaxStateMsgNO)
	p2p := p2pnetwork.NewSimpleP2pLib(id, ch)
	se := &StateEngine{
		NodeID:          id,
		CurViewID:       0,
		CurSequence:     0,
		LasExeSeq:       0,
		MiniSeq:         0,
		MaxSeq:          0 + CheckPointK,
		Timer:           newRequestTimer(),
		p2pWire:         p2p,
		MsgChan:         ch,
		nodeChan:        cChan,
		directReplyChan: rChan,
		msgLogs:         make(map[int64]*NormalLog),
		checks:          make(map[int64]*CheckPoint),
		cliRecord:       make(map[string]*ClientRecord),
		sCache:          NewVCCache(),
	}
	se.PrimaryID = se.CurViewID % message.TotalNodeNO
	return se
}

func (s *StateEngine) StartConsensus(sig chan interface{}) {
	s.nodeStatus = Serving
	//defer func() {
	//	if r := recover(); r != nil {
	//		sig <- r
	//	}
	//}()

	for {
		select {
		case <-s.Timer.C:
			s.ViewChange()
		case conMsg := <-s.MsgChan:
			switch conMsg.Typ {
			case message.MTRequest,
				message.MTPrePrepare,
				message.MTPrepare,
				message.MTCommit:
				if s.nodeStatus != Serving {
					fmt.Println("node is not in service status now......")
					continue
				}
				if err := s.procConsensusMsg(conMsg); err != nil {
					fmt.Print(err)
				}
			case message.MTCheckpoint,
				message.MTViewChange,
				message.MTNewView:
				if err := s.procManageMsg(conMsg); err != nil {
					fmt.Print(err)
				}
			}
		}
	}
}

/*
	In the pre-Prepare phase, the primary assigns a sequence number, n, to the Request, multicasts a pre- Prepare
message with   piggybacked to all the backups, and appends the message to its log. The message has the
form <<PRE-PREPARE, v, n, d>, m> where v indicates the view in which the message is being sent, m is client’s Request
message, and  d ism’s digest.
	Requests are not included in pre-Prepare messages to keep them small. This is important because pre-Prepare
messages are used as a proof that the Request was assigned sequence number   in view   in view changes. Additionally,
it decouples the protocol to totally order requests from the protocol to transmit the Request to the replicas;
allowing us to use a transport optimized for small messages for protocol messages and a transport optimized for
large messages for large requests.
*/

func (s *StateEngine) checkClientRecord(request *message.Request) (*ClientRecord, error) {
	client, ok := s.cliRecord[request.ClientID]
	if !ok {
		client = NewClientRecord()
		s.cliRecord[request.ClientID] = client
		fmt.Printf("======>[Primary] New Client ID:%s\n", request.ClientID)
	}

	if request.TimeStamp < client.LastReplyTime {
		rp, ok := client.Reply[request.TimeStamp]
		if ok {
			fmt.Printf("======>[Primary] direct reply:%d\n", rp.SeqID)
			s.directReplyChan <- rp
			return nil, nil
		}
		return nil, fmt.Errorf("======>[Primary] it's a old operation Request")
	}
	return client, nil
}

func (s *StateEngine) getOrCreateLog(seq int64) *NormalLog {
	log, ok := s.msgLogs[seq]
	if !ok {
		log = NewNormalLog()
		s.msgLogs[seq] = log
	}
	return log
}

func (s *StateEngine) InspireConsensus(request *message.Request) error {
	s.CurSequence++
	newSeq := s.CurSequence
	request.SeqID = newSeq
	client, err := s.checkClientRecord(request)
	if err != nil || client == nil {
		return err
	}
	client.saveRequest(request)
	cMsg := message.CreateConMsg(message.MTRequest, request)
	if err := s.p2pWire.BroadCast(cMsg); err != nil {
		return err
	}
	dig := message.Digest(request)
	ppMsg := &message.PrePrepare{
		ViewID:     s.CurViewID,
		SequenceID: newSeq,
		Digest:     dig,
	}

	log := s.getOrCreateLog(newSeq)
	log.PrePrepare = ppMsg
	log.clientID = request.ClientID
	cMsg = message.CreateConMsg(message.MTPrePrepare, ppMsg)
	if err := s.p2pWire.BroadCast(cMsg); err != nil {
		return err
	}
	log.Stage = PrePrepared
	fmt.Printf("======>[Primary]Consensus broadcast message(%d)\n", newSeq)
	return nil
}

/*
A backup accepts a pre-Prepare message provided:
	1. the signatures in the Request and the pre-Prepare message are correct and d is the digest for m ;
	2. it is in view v;
	3. it has not accepted a pre-Prepare message for view v and sequence number n containing a different digest;
	4. the sequence number in the pre-Prepare message is between a low water mark, h, and a high water mark H,

The last condition prevents a faulty primary from exhausting the space of sequence numbers by selecting a very large one.

	If backup i accepts the <<PRE-PREPARE, v, n, d>, m> message, it enters the Prepare phase by multicasting a
<PREPARE, v, n, d, i>  message to all other replicas and adds both messages to its log. Otherwise, it does nothing.
*/

func (s *StateEngine) rawRequest(request *message.Request) (err error) {
	//TODO:: check signature of Request
	client, err := s.checkClientRecord(request)
	if err != nil || client == nil {
		return err
	}
	log := s.getOrCreateLog(request.SeqID)
	log.clientID = request.ClientID
	client.saveRequest(request)
	s.Timer.tick()
	return nil
}

/*

	Like PRE-PREPAREs, the PREPARE and COMMIT messages sent in the other phases also contain n and v. A replica
only accepts one of these messages provided that it is in view v; that it can verify the authenticity of the message;
and that n is between a low water mark h and a high water mark H.
	A backup i accepts the PRE-PREPARE message provided (in addition to the conditions above) it has not accepted
a PRE-PREPARE for view v and sequence number n containing a different digest.If a backup i accepts the PRE-PREPARE and
it has request m in its log, it enters the prepare phase by multicasting a PREPARE message with m’s digest to all other
replicas; in addition, it adds both the PRE-PREPARE and PREPARE messages to its log.
*/
func (s *StateEngine) idle2PrePrepare(ppMsg *message.PrePrepare) (err error) {

	fmt.Printf("======>[idle2PrePrepare]Current sequence[%d]\n", ppMsg.SequenceID)

	//TODO:: check signature of of pre-Prepare message
	//TODO:: check digest of pre-Prepare message

	if ppMsg.ViewID != s.CurViewID {
		return fmt.Errorf("======>[idle2PrePrepare] invalid view id Msg=%d state=%d\n", ppMsg.ViewID, s.CurViewID)
	}

	if ppMsg.SequenceID > s.MaxSeq || ppMsg.SequenceID < s.MiniSeq {
		return fmt.Errorf("======>[idle2PrePrepare] sequence no[%d] invalid[%d~%d]\n", ppMsg.SequenceID, s.MiniSeq, s.MaxSeq)
	}

	log := s.getOrCreateLog(ppMsg.SequenceID)

	if log.Stage != Idle {
		return fmt.Errorf("invalid stage[current %s] when to prePrepared", log.Stage)
	}
	if log.PrePrepare != nil {
		if log.PrePrepare.Digest != ppMsg.Digest {
			return fmt.Errorf("pre-Prepare message in same v-n but not same digest")
		} else {
			fmt.Println("======>[idle2PrePrepare] duplicate pre-Prepare message")
			return
		}
	}
	prepare := &message.Prepare{
		ViewID:     s.CurViewID,
		SequenceID: ppMsg.SequenceID,
		Digest:     ppMsg.Digest,
		NodeID:     s.NodeID,
	}
	cMsg := message.CreateConMsg(message.MTPrepare, ppMsg)
	if err := s.p2pWire.BroadCast(cMsg); err != nil {
		return err
	}
	log.PrePrepare = ppMsg
	log.Prepare[s.NodeID] = prepare
	log.Stage = PrePrepared
	fmt.Printf("======>[idle2PrePrepare] Consensus status is [%s] seq=%d\n", log.Stage, ppMsg.SequenceID)
	return nil
}

/*
	A replica (including the primary) accepts Prepare messages and adds them to its log provided their signatures
are correct, their view number equals the replica’s current view, and their sequence number is between h and H.

	We define the predicate prepared (m, v, n, i) to be true if and only if replica i has inserted in its log:
	1. the Request m, a pre-Prepare for m in view  v with sequence number n,
	2. 2f prepares from different backups that match the pre-Prepare.

	The replicas verify whether the prepares match the pre-Prepare by checking that they have the same :
	1. view,
	2. sequence number,
	3. and digest.
*/

func (s *StateEngine) prePrepare2Prepare(prepare *message.Prepare) (err error) {

	fmt.Printf("======>[prePrepare2Prepare]Current sequence[%d]\n", prepare.SequenceID)

	//TODO::signature check
	//fmt.Printf("Verify Prepare message digest:%s\n", Prepare.Digest)
	if prepare.ViewID != s.CurViewID {
		return fmt.Errorf("======>[prePrepare2Prepare]:=>invalid view id Msg=%d state=%d\n", prepare.ViewID, s.CurViewID)
	}

	if prepare.SequenceID > s.MaxSeq || prepare.SequenceID < s.MiniSeq {
		return fmt.Errorf("======>[prePrepare2Prepare]:=>sequence no[%d] invalid[%d~%d]\n", prepare.SequenceID, s.MiniSeq, s.MaxSeq)
	}

	log, ok := s.msgLogs[prepare.SequenceID]
	if !ok {
		return fmt.Errorf("======>[prePrepare2Prepare]:=>havn't got log for message(%d) yet", prepare.SequenceID)
	}
	if log.Stage != PrePrepared {
		return fmt.Errorf("======>[prePrepare2Prepare] current[seq=%d] state isn't PrePrepared:[%s]\n", prepare.SequenceID, log.Stage)
	}

	ppMsg := log.PrePrepare
	if ppMsg == nil {
		return fmt.Errorf("======>[prePrepare2Prepare]:=>havn't got pre-Prepare message(%d) yet", prepare.SequenceID)
	}

	if ppMsg.ViewID != prepare.ViewID ||
		ppMsg.SequenceID != prepare.SequenceID ||
		ppMsg.Digest != prepare.Digest {
		return fmt.Errorf("[Prepare]:=>not same with pre-Prepare message")
	}
	log.Prepare[prepare.NodeID] = prepare
	if len(log.Prepare) < 2*message.MaxFaultyNode { //not different replica, just simple no
		return nil
	}

	fmt.Printf("======>[prePrepare2Prepare] Consensus status is [%s] seq=%d\n", log.Stage, prepare.SequenceID)

	commit := &message.Commit{
		ViewID:     s.CurViewID,
		SequenceID: prepare.SequenceID,
		Digest:     prepare.Digest,
		NodeID:     s.NodeID,
	}
	cMsg := message.CreateConMsg(message.MTCommit, commit)
	if err := s.p2pWire.BroadCast(cMsg); err != nil {
		return err
	}
	log.Commit[s.NodeID] = commit
	log.Stage = Prepared
	return
}

/*
	Replica i multicasts a <COMMIT, v, n, D(m), i> to the other replicas when prepared (m, v, n, i) become true.
This starts the Commit phase. Replicas accept Commit messages and insert them in their log provided they are properly
signed, the view number in the message is equal to the replica’s current view, and the sequence number is between h and H

	We define the committed and committed-local predicates as follows: committed(m, v, n) is true if and only
if prepared(m, v, n, i) is true for all i in some set of f + 1 non-faulty replicas; and committed-local(m, v, n, i) is
true if and only if prepared(m, v, n, i) is true and i has accepted 2f + 1 commits (possibly including its own) from
different replicas that match the pre-Prepare for m; a Commit matches a pre-Prepare if they have the same view, sequence
number, and digest.
	The Commit phase ensures the following invariant: if committed-local(m, v, n, i) is true for some non-faulty i
then committed(m, v, n, i) is true. This invariant and the view-change protocol described in Section 4.4 ensure that
non-faulty replicas agree on the sequence numbers of requests that Commit locally even if they Commit in different views
at each replica. Furthermore, it ensures that any Request that commits locally at a non-faulty replica will Commit at
 f + 1 or more non-faulty replicas eventually.
	We do not rely on ordered message delivery, and therefore it is possible for a replica to Commit requests out
of order. This does not matter since it keeps the pre- Prepare, Prepare, and Commit messages logged until the
corresponding Request can be executed.
*/

func (s *StateEngine) prepare2Commit(commit *message.Commit) (err error) {
	fmt.Printf("======>[prepare2Commit] Current sequence[%d]\n", commit.SequenceID)

	//TODO:: Commit is properly signed
	//fmt.Printf("Verify Commit message digest:%s\n", Commit.Digest)
	if commit.ViewID != s.CurViewID {
		return fmt.Errorf("======>[prepare2Commit]  invalid view id Msg=%d state=%d\n", commit.ViewID, s.CurViewID)
	}

	if commit.SequenceID > s.MaxSeq || commit.SequenceID < s.MiniSeq {
		return fmt.Errorf("======>[prepare2Commit] sequence no[%d] invalid[%d~%d]\n",
			commit.SequenceID, s.MiniSeq, s.MaxSeq)
	}

	log, ok := s.msgLogs[commit.SequenceID]
	if !ok {
		return fmt.Errorf("======>[prePrepare2Prepare]:=>havn't got log for message(%d) yet", commit.SequenceID)
	}
	if log.Stage != Prepared {
		return fmt.Errorf("======>[prePrepare2Prepare] current[seq=%d] state isn't PrePrepared:[%s]\n", commit.SequenceID, log.Stage)
	}

	ppMsg := log.PrePrepare
	if ppMsg == nil {
		return fmt.Errorf("======>[prePrepare2Prepare]:=>havn't got pre-Prepare message(%d) yet", commit.SequenceID)
	}

	if ppMsg.ViewID != commit.ViewID ||
		ppMsg.SequenceID != commit.SequenceID ||
		ppMsg.Digest != commit.Digest {
		return fmt.Errorf("[Prepare]:=>not same with pre-Prepare message")
	}
	log.Commit[commit.NodeID] = commit
	if len(log.Commit) < 2*message.MaxFaultyNode+1 {
		return nil
	}
	log.Stage = Committed
	s.Timer.tack()
	fmt.Printf("======>[prepare2Commit] Consensus status is [%s] seq=%d and timer stop\n", log.Stage, commit.SequenceID)

	//TODO::should execute request with smallest  sequence no, current committed sequence may not be the smallest one.
	request, ok := s.cliRecord[log.clientID].Request[commit.SequenceID]
	if !ok {
		return fmt.Errorf("no raw request for such seq[%d]", commit.SequenceID)
	}
	exeParam := &message.RequestRecord{
		Request:    request,
		PrePrepare: ppMsg,
	}
	//TODO::Check the reply whose sequence is smaller than current sequence.
	s.nodeChan <- exeParam
	return
}

func (s *StateEngine) procConsensusMsg(msg *message.ConMessage) (err error) {
	s.Timer.Reset(StateTimerOut)

	fmt.Printf("\n======>[procConsensusMsg] Consesus message signature:(%s)\n", msg.Sig)

	switch msg.Typ {

	case message.MTRequest:
		request := &message.Request{}
		if err := json.Unmarshal(msg.Payload, request); err != nil {
			return fmt.Errorf("======>[procConsensusMsg] Invalid[%s] request message[%s]\n", err, msg)
		}
		return s.rawRequest(request)
	case message.MTPrePrepare:
		prePrepare := &message.PrePrepare{}
		if err := json.Unmarshal(msg.Payload, prePrepare); err != nil {
			return fmt.Errorf("======>[procConsensusMsg] Invalid[%s] pre-Prepare message[%s]\n", err, msg)
		}
		return s.idle2PrePrepare(prePrepare)

	case message.MTPrepare:
		prepare := &message.Prepare{}
		if err := json.Unmarshal(msg.Payload, prepare); err != nil {
			return fmt.Errorf("======>[procConsensusMsg]invalid[%s] Prepare message[%s]\n", err, msg)
		}
		return s.prePrepare2Prepare(prepare)

	case message.MTCommit:
		commit := &message.Commit{}
		if err := json.Unmarshal(msg.Payload, commit); err != nil {
			return fmt.Errorf("======>[procConsensusMsg] invalid[%s] Commit message[%s]\n", err, msg)
		}
		return s.prepare2Commit(commit)
	}
	return
}

func (s *StateEngine) procManageMsg(msg *message.ConMessage) (err error) {
	switch msg.Typ {

	case message.MTCheckpoint:
		checkpoint := &message.CheckPoint{}
		if err := json.Unmarshal(msg.Payload, checkpoint); err != nil {
			return fmt.Errorf("======>[procConsensusMsg] invalid[%s]checkpoint message[%s]\n", err, msg)
		}
		return s.checkingPoint(checkpoint)

	case message.MTViewChange:
		vc := &message.ViewChange{}
		if err := json.Unmarshal(msg.Payload, vc); err != nil {
			return fmt.Errorf("======>[procConsensusMsg] invalid[%s]ViewChange message[%s]\n", err, msg)
		}
		return s.procViewChange(vc)

	case message.MTNewView:
		vc := &message.NewView{}
		if err := json.Unmarshal(msg.Payload, vc); err != nil {
			return fmt.Errorf("======>[procConsensusMsg] invalid[%s] didiViewChange message[%s]\n", err, msg)
		}
		return s.didChangeView(vc)
	}
	return nil
}
