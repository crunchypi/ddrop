package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/crunchypi/ddrop/service/api"
)

func main() {
	flag.Usage = func() {
		s := "---------------------------------------------------\n"
		s += "ddrop\n"
		s += "For benchmarking distributed recommendation systems.\n"
		s += "See https://github.com/crunchypi/ddrop\n"
		s += "\n"
		s += "This build is for starting the http server which is \n"
		s += "used to interface with the system. See the above \n"
		s += "link for endpoints and documentation.\n"
		s += "\n"
		s += "Args:\n"
		fmt.Fprintf(os.Stderr, s)
		flag.PrintDefaults()
	}

	addr := flag.String("addr", "localhost:80",
		"Specify the http server address",
	)
	ioTimeout := flag.Int("io-timeout", 10,
		"Specify in seconds the http server's read/write timeout",
	)

    
	flag.Parse()

	ctx, _ := signal.NotifyContext(
		context.Background(),
		syscall.SIGKILL,
		syscall.SIGTERM,
		syscall.SIGINT,
	)
    _, err := api.StartServer(api.StartServerArgs{
		Addr:                   *addr,
		Ctx:                    ctx,
		ReadTimeout:            time.Second * time.Duration(*ioTimeout),
		WriteTimeout:           time.Second * time.Duration(*ioTimeout),
		UpdateFrequencyAddrSet: time.Second * 10,
		OnStart: func() {
			log.Printf("started listening on addr '%s'\n", *addr)
		},
	})

    if err != nil {
        log.Fatal(err)
        
    }

	log.Println("\nstopped")
}
