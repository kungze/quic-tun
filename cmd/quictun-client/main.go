package main

import (
	"os"
	"runtime"

	"github.com/kungze/quic-tun/client"
)

func main() {
	if len(os.Getenv("GOMAXPROCS")) == 0 {
		runtime.GOMAXPROCS(runtime.NumCPU())
	}

	client.NewApp("quictun-client").Run()
}
