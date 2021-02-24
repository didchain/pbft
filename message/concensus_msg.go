package message

import (
	"encoding/json"
	"fmt"
)

type ConMessage struct {
	Typ     MType  `json:"type"`
	Sig     string `json:"sig"`
	From    string `json:"from"`
	Payload []byte `json:"payload"`
}

func (cm *ConMessage) String() string {
	return fmt.Sprintf("\n======Consensus Messagetype======"+
		"\ntype:%40s"+
		"\nsig:%40s"+
		"\npayload:%d"+
		"\n<------------------>",
		cm.Typ.String(),
		cm.Sig,
		len(cm.Payload))
}

func (cm *ConMessage) Verify() bool {
	//hash := HASH(cm.Payload)
	//return cm.From == Revert(hash, cm.Sig)
	return true
}

func CreateConMsg(t MType, msg interface{}) *ConMessage {
	data, e := json.Marshal(msg)
	if e != nil {
		return nil
	}

	sig := fmt.Sprintf("consensus message[%s]", t)
	consMsg := &ConMessage{
		Typ:     t,
		Sig:     sig,
		Payload: data,
	}
	return consMsg
}

type RequestRecord struct {
	*PrePrepare
	*Request
}

type PrePrepare struct {
	ViewID     int64  `json:"viewID"`
	SequenceID int64  `json:"sequenceID"`
	Digest     string `json:"digest"`
}

type PrepareMsg map[int64]*Prepare
type Prepare struct {
	ViewID     int64  `json:"viewID"`
	SequenceID int64  `json:"sequenceID"`
	Digest     string `json:"digest"`
	NodeID     int64  `json:"nodeID"`
}

type Commit struct {
	ViewID     int64  `json:"viewID"`
	SequenceID int64  `json:"sequenceID"`
	Digest     string `json:"digest"`
	NodeID     int64  `json:"nodeID"`
}

type CheckPoint struct {
	SequenceID int64  `json:"sequenceID"`
	Digest     string `json:"digest"`
	ViewID     int64  `json:"viewID"`
	NodeID     int64  `json:"nodeID"`
}
type PTuple struct {
	PPMsg *PrePrepare `json:"pre-prepare"`
	PMsg  PrepareMsg  `json:"prepare"`
}

type ViewChange struct {
	NewViewID int64                 `json:"newViewID"`
	LastCPSeq int64                 `json:"lastCPSeq"`
	NodeID    int64                 `json:"nodeID"`
	CMsg      map[int64]*CheckPoint `json:"cMsg"`
	PMsg      map[int64]*PTuple     `json:"pMsg"`
}

func (vc *ViewChange) Digest() string {
	return fmt.Sprintf("this is digest for[%d-%d]", vc.NewViewID, vc.LastCPSeq)
}

type OMessage map[int64]*PrePrepare

func (m OMessage) EQ(msg OMessage) bool {
	//return HASH(m) == HASH(msg)
	return true
}

type VMessage map[int64]*ViewChange
type NewView struct {
	NewViewID int64    `json:"newViewID"`
	VMsg      VMessage `json:"vMSG"`
	OMsg      OMessage `json:"oMSG"`
	NMsg      OMessage `json:"nMSG"`
}
