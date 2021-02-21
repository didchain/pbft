package consensus

import "github.com/didchain/PBFT/message"

type NormalLog struct {
	clientID   string                     `json:"clientID"`
	Stage      Stage                      `json:"Stage"`
	PrePrepare *message.PrePrepare        `json:"PrePrepare"`
	Prepare    map[int64]*message.Prepare `json:"Prepare"`
	Commit     map[int64]*message.Commit  `json:"Commit"`
}

func NewNormalLog() *NormalLog {
	nl := &NormalLog{
		Stage:      Idle,
		PrePrepare: nil,
		Prepare:    make(map[int64]*message.Prepare),
		Commit:     make(map[int64]*message.Commit),
	}
	return nl
}
