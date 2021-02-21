package main

import (
	"encoding/json"
	"fmt"
	"github.com/didchain/PBFT/message"
	"net"
	"os"
	"strconv"
	"sync"
	"time"
)

func request(conn *net.UDPConn, wg *sync.RWMutex) {
	for {
		wg.Lock()
		primaryID, _ := strconv.Atoi(os.Args[1])
		rAddr := net.UDPAddr{
			Port: message.PortByID(int64(primaryID)),
		}

		r := &message.Request{
			TimeStamp: time.Now().Unix(),
			ClientID:  "Client's address",
			Operation: "<READ TX FROM POOL>",
		}

		bs, err := json.Marshal(r)
		if err != nil {
			panic(err)
		}

		n, err := conn.WriteToUDP(bs, &rAddr)
		if err != nil || n == 0 {
			panic(err)
		}
		fmt.Println("Send request success!:=>")
	}
}

func normalCaseOperation(roundSize int) {
	fmt.Println("start test.....")
	lclAddr := net.UDPAddr{
		Port: 8088,
	}
	conn, err := net.ListenUDP("udp4", &lclAddr)
	if err != nil {
		panic(err)
	}

	defer conn.Close()

	locker := &sync.RWMutex{}
	go request(conn, locker)
	waitBuffer := make([]byte, 1024)
	var counter = 0
	var curSeq int64 = 0
	for {
		n, _, err := conn.ReadFromUDP(waitBuffer)
		if err != nil {
			panic(err)
		}
		//fmt.Printf("Client Read[%d] Reply from[%s]:\n", n, rAddr.String())

		re := &message.Reply{}
		if err := json.Unmarshal(waitBuffer[:n], re); err != nil {
			panic(err)
		}

		if curSeq > re.SeqID {
			continue
		}

		counter++
		if counter >= 2 {
			fmt.Printf("Consensus(seq=%d) operation(%d) success!\n", curSeq, roundSize)
			locker.Unlock()
			counter = 0
			roundSize--
			curSeq = re.SeqID + 1
		}
		if roundSize <= 0 {
			fmt.Println("Test case finished")
			os.Exit(0)
		}
	}
}

func main() {
	//normalCaseOperation(51)
	//normalCaseOperation(21)
	//normalCaseOperation(30)
	//normalCaseOperation(100)
	normalCaseOperation(1)
}
