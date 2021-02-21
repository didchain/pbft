package p2pnetwork

import (
	"encoding/json"
	"fmt"
	"github.com/didchain/PBFT/message"
	"io"
	"net"
	"time"
)

var nodeList = []int64{0, 1, 2, 3}

type P2pNetwork interface {
	BroadCast(v interface{}) error
	SendToNode(nodeID int64, v interface{}) error
}

type SimpleP2p struct {
	SrvBub  *net.TCPListener
	Peers   map[string]*net.TCPConn
	MsgChan chan<- *message.ConMessage
}

func NewSimpleP2pLib(id int64, msgChan chan<- *message.ConMessage) P2pNetwork {

	port := message.PortByID(id)
	s, err := net.ListenTCP("tcp4", &net.TCPAddr{
		Port: port,
	})

	if err != nil {
		panic(err)
	}

	sp := &SimpleP2p{
		SrvBub:  s,
		Peers:   make(map[string]*net.TCPConn),
		MsgChan: msgChan,
	}
	go sp.monitor()
	for _, pid := range nodeList {
		if pid == id {
			continue
		}

		rPort := message.PortByID(pid)
		conn, err := net.DialTCP("tcp", nil, &net.TCPAddr{Port: rPort})
		if err != nil {
			fmt.Printf("\nnode [%d] is not valid currently\n", pid)
			continue
		}
		sp.Peers[conn.RemoteAddr().String()] = conn
		fmt.Printf("node [%d] connected=[%s=>%s]\n", pid, conn.LocalAddr().String(), conn.RemoteAddr().String())
		go sp.waitData(conn)
	}
	return sp
}

func (sp *SimpleP2p) monitor() {
	fmt.Printf("===>P2p node is waiting at:%s\n", sp.SrvBub.Addr().String())
	for {
		conn, err := sp.SrvBub.AcceptTCP()
		if err != nil {
			fmt.Printf("P2p network accept err:%s\n", err)
			if err == io.EOF {
				fmt.Printf("Remove peer node%s\n", conn.RemoteAddr().String())
				delete(sp.Peers, conn.RemoteAddr().String())
			}
			continue
		}

		sp.Peers[conn.RemoteAddr().String()] = conn
		fmt.Printf("connection create [%s->%s]\n", conn.RemoteAddr().String(), conn.LocalAddr().String())
		go sp.waitData(conn)
	}
}

func (sp *SimpleP2p) waitData(conn *net.TCPConn) {
	buf := make([]byte, 2048)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			fmt.Printf("P2p network capture data err:%s\n", err)
			if err == io.EOF {
				fmt.Printf("Remove peer node%s\n", conn.RemoteAddr().String())
				delete(sp.Peers, conn.RemoteAddr().String())
				return

			}
			continue
		}
		conMsg := &message.ConMessage{}
		if err := json.Unmarshal(buf[:n], conMsg); err != nil {
			fmt.Println(string(buf[:n]))
			panic(err)
		}
		sp.MsgChan <- conMsg
	}
}

func (sp *SimpleP2p) BroadCast(v interface{}) error {
	if v == nil {
		return fmt.Errorf("empty msg body")
	}
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	for name, conn := range sp.Peers {
		_, err := conn.Write(data)
		if err != nil {
			fmt.Printf("write to node[%s] err:%s\n", name, err)
		}
	}
	time.Sleep(300 * time.Millisecond)
	return nil
}

func (sp *SimpleP2p) SendToNode(nodeID int64, v interface{}) error {
	//TODO:: single point message
	return sp.BroadCast(v)
}
