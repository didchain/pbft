package main

import (
	"fmt"
	"github.com/didchain/PBFT/node"
	"os"
	"os/signal"
	"strconv"
	"syscall"
)

func main() {
	if len(os.Args) < 2 {
		panic("usage: input id")
	}

	id, _ := strconv.Atoi(os.Args[1])
	node := node.NewNode(int64(id))
	go node.Run()

	sigCh := make(chan os.Signal, 1)

	signal.Notify(sigCh,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)

	pid := strconv.Itoa(os.Getpid())

	fmt.Printf("\n===>PBFT Demo PID:%s\n", pid)
	fmt.Println()
	fmt.Println()
	fmt.Println("==============================================>")
	fmt.Println("*                                             *")
	fmt.Println("*     Practical Byzantine Fault Tolerance     *")
	fmt.Println("*                                             *")
	fmt.Println("<==============================================")
	fmt.Println()
	fmt.Println()
	sig := <-sigCh
	fmt.Printf("Finish by signal:===>[%s]\n", sig.String())
}
