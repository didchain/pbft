package consensus

import (
	"github.com/didchain/PBFT/message"
	"time"
)

type ClientRecord struct {
	LastReplyTime int64                      `json:"lastReply"`
	Request       map[int64]*message.Request `json:"Request"`
	Reply         map[int64]*message.Reply   `json:"Reply"`
}

func NewClientRecord() *ClientRecord {
	cr := &ClientRecord{
		LastReplyTime: -1,
		Request:       make(map[int64]*message.Request),
		Reply:         make(map[int64]*message.Reply),
	}

	return cr
}

func (cr *ClientRecord) getRequest(seq int64) (*message.Request, bool) {
	r, ok := cr.Request[seq]
	return r, ok
}

func (cr *ClientRecord) saveRequest(r *message.Request) {
	cr.Request[r.SeqID] = r
}

func (cr *ClientRecord) getReply(time int64) (*message.Reply, bool) {
	r, ok := cr.Reply[time]
	return r, ok
}

func (cr *ClientRecord) saveReply(reply *message.Reply) {
	cr.Reply[reply.Timestamp] = reply
	cr.LastReplyTime = time.Now().Unix()
}
