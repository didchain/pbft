package message

import (
	"fmt"
)

type MType int16

const (
	MTPrePrepare MType = iota
	MTRequest
	MTPrepare
	MTCommit
	MTCheckpoint
	MTViewChange
	MTNewView
)
const MaxFaultyNode = 1
const TotalNodeNO = 3*MaxFaultyNode + 1

func Digest(v interface{}) string {
	return "0x111111"
}

func PortByID(id int64) int {
	return 30000 + int(id)
}

func (mt MType) String() string {
	switch mt {
	case MTPrePrepare:
		return "PrePrepare"

	case MTPrepare:
		return "Prepare"

	case MTCommit:
		return "Commit"

	case MTCheckpoint:
		return "Checkpoint"
	}
	return "Unknown"
}

type Request struct {
	SeqID     int64  `json:"sequenceID"`
	TimeStamp int64  `json:"timestamp"`
	ClientID  string `json:"clientID"`
	Operation string `json:"operation"`
}

func (r *Request) String() string {
	return fmt.Sprintf("\n clientID:%20s"+
		"\n time:%d"+
		"\n operation:%s",
		r.ClientID,
		r.TimeStamp,
		r.Operation)
}

type Reply struct {
	SeqID     int64  `json:"sequenceID"`
	ViewID    int64  `json:"viewID"`
	Timestamp int64  `json:"timestamp"`
	ClientID  string `json:"clientID"`
	NodeID    int64  `json:"nodeID"`
	Result    string `json:"result"`
}
