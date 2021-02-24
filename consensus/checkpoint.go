package consensus

import (
	"fmt"
	"github.com/didchain/PBFT/message"
)

/*
	Generating these proofs after executing every operation would be expensive. Instead, they are generated periodically,
when a Request with a sequence number di- visible by some constant (e.g., 100) is executed. We will refer to the states
produced by the execution of these re- quests as checkpoints and we will say that a checkpoint with a proof is a stable checkpoint.

	A replica maintains several logical copies of the service state: the last stable checkpoint, zero or more
checkpoints that are not stable, and a current state. Copy-on-write techniques can be used to reduce the space overhead
to store the extra copies of the state, as discussed in Section 6.3.
	The proof of correctness for a checkpoint is generated as follows. When a replica produces a checkpoint, it
multicasts a message <CHECKPOINT, n, d, i> to the other replicas, where n is the sequence number of the last Request
whose execution is reflected in the state and d is the digest of the state. Each replica collects checkpoint messages
in its log until it has 2f + 1 of them for sequence number n with the same digest signed by different replicas
(including possibly its own such message). These 2f + 1 messages are the proof of correctness for the checkpoint.
	A checkpoint with a proof becomes stable and the replica discards all pre-Prepare, Prepare, and Commit messages
with sequence number less than or equal to n from its log; it also discards all earlier checkpoints and checkpoint messages.
	Computing the proofs is efficient because the digest can be computed using incremental cryptography [1] as
discussed in Section 6.3, and proofs are generated rarely.
	The checkpoint protocol is used to advance the low and high water marks (which limit what messages will be accepted).
The low-water mark h is equal to the sequence number of the last stable checkpoint. The high water mark H = h + k, where
k is big enough so that replicas do not stall waiting for a checkpoint to become stable. For example, if checkpoints
are taken every 100 requests, k might be 200.
*/
type CheckPoint struct {
	Seq      int64                         `json:"sequence"`
	Digest   string                        `json:"digest"`
	IsStable bool                          `json:"isStable"`
	ViewID   int64                         `json:"viewID"`
	CPMsg    map[int64]*message.CheckPoint `json:"checks"`
}

func NewCheckPoint(sq, vi int64) *CheckPoint {
	cp := &CheckPoint{
		Seq:      sq,
		IsStable: false,
		ViewID:   vi,
		CPMsg:    make(map[int64]*message.CheckPoint),
	}
	return cp
}

func (s *StateEngine) ResetState(reply *message.Reply) {
	s.msgLogs[reply.SeqID].Stage = Idle
	s.LasExeSeq = reply.SeqID

	if s.CurSequence%CheckPointInterval == 0 {
		fmt.Printf("======>[ResetState]Need to create check points(%d)\n", s.CurSequence)
		go s.createCheckPoint(s.CurSequence)
	}
	s.cliRecord[reply.ClientID].saveReply(reply)
}

func (s *StateEngine) createCheckPoint(sequence int64) {
	msg := &message.CheckPoint{
		SequenceID: sequence,
		NodeID:     s.NodeID,
		ViewID:     s.CurViewID,
		Digest:     fmt.Sprintf("checkpoint message for [seq(%d)]", sequence),
	}

	cp := NewCheckPoint(sequence, s.CurViewID)
	cp.Digest = fmt.Sprintf("check point message<%d, %d>", s.NodeID, sequence)
	cp.CPMsg[s.NodeID] = msg
	s.checks[sequence] = cp

	fmt.Printf("======>[createCheckPoint] Broadcast check point message<%d, %d>\n", s.NodeID, sequence)
	consMsg := message.CreateConMsg(message.MTCommit, msg)
	err := s.p2pWire.BroadCast(consMsg)
	if err != nil {
		fmt.Println(err)
	}
}

func (s *StateEngine) checkingPoint(msg *message.CheckPoint) error {
	cp, ok := s.checks[msg.SequenceID]
	if !ok {
		cp = NewCheckPoint(msg.SequenceID, s.CurViewID)
		s.checks[msg.SequenceID] = cp
	}
	cp.CPMsg[msg.NodeID] = msg
	s.runCheckPoint(msg.SequenceID)
	return nil
}

func (s *StateEngine) runCheckPoint(seq int64) {
	cp, ok := s.checks[seq]
	if !ok {
		return

	}
	if len(cp.CPMsg) < 2*message.MaxFaultyNode+1 {
		fmt.Printf("======>[checkingPoint] message counter:[%d]\n", len(cp.CPMsg))
		return
	}
	if cp.IsStable {
		fmt.Printf("======>[checkingPoint] Check Point for [%d] has confirmed\n", cp.Seq)
		return
	}

	fmt.Println("======>[checkingPoint] Start to clean the old message data......")
	cp.IsStable = true
	for id, log := range s.msgLogs {
		if id > cp.Seq {
			continue
		}
		log.PrePrepare = nil
		log.Commit = nil
		delete(s.msgLogs, id)
		fmt.Printf("======>[checkingPoint] Delete log message:CPseq=%d  clientID=%s\n", id, log.clientID)
	}

	for id, cps := range s.checks {
		if id >= cp.Seq {
			continue
		}
		cps.CPMsg = nil
		delete(s.checks, id)
		fmt.Printf("======>[checkingPoint] Delete Checkpoint:seq=%d stable=%t\n", id, cps.IsStable)
	}

	s.MiniSeq = cp.Seq
	s.MaxSeq = s.MiniSeq + CheckPointK
	s.lastCP = cp
	fmt.Printf("======>[checkingPoint] Success in Checkpoint forwarding[(%d, %d)]......\n", s.MiniSeq, s.MaxSeq)
}
