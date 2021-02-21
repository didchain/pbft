package message

import (
	"encoding/json"
	"fmt"
)

type ConMessage struct {
	Typ     MType  `json:"type"`
	Sig     string `json:"sig"`
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
	NodeID     int64  `json:"nodeID"`
}

type ViewChange struct {
	NewViewID int64             `json:"newViewID"`
	LastCPSeq int64             `json:"lastCPSeq"`
	NodeID    int64             `json:"nodeID"`
	CMsg      []*CTuple         `json:"cMsg"`
	PMsg      map[int64]*PTuple `json:"pMsg"`
	QMsg      map[int64]*QTuple `json:"qMsg"`
}

func (vc *ViewChange) Digest() string {
	return fmt.Sprintf("this is digest for[%d-%d]", vc.NewViewID, vc.LastCPSeq)
}

type ViewChangeACK struct {
	NewViewID int64  `json:"newViewID"`
	NodeI     int64  `json:"nodeI"`
	NodeJ     int64  `json:"nodeJ"`
	Digest    string `json:"digest"`
}

type PTuple struct {
	ViewID     int64 `json:"viewID"`
	SequenceID int64 `json:"sequenceID"`
	NodeID     int64 `json:"nodeID"`
}

type QTuple struct {
	ViewID     int64 `json:"viewID"`
	SequenceID int64 `json:"sequenceID"`
	NodeID     int64 `json:"nodeID"`
}

type CTuple struct {
	SequenceID int64  `json:"sequenceID"`
	Digest     string `json:"digest"`
}

type VTuple struct {
	NodeID int64  `json:"nodeID"`
	Digest string `json:"digest"`
}

type XTuple struct {
	SequenceID       int64             `json:"sequenceID"`
	Digest           string            `json:"digest"`
	SelectedRequests map[int64]Request `json:"requests"`
}

type NewView struct {
	NewViewID int64     `json:"newViewID"`
	VMsg      []*VTuple `json:"vMSG"`
	XMsg      *XTuple   `json:"xMSG"`
}
