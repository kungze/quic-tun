package main

import (
	"os"
	"runtime"

	"github.com/kungze/quic-tun/server"
)

func main() {
	if len(os.Getenv("GOMAXPROCS")) == 0 {
		runtime.GOMAXPROCS(runtime.NumCPU())
	}

	server.NewApp("quictun-server").Run()
}
