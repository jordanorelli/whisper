package main

import (
	"flag"
	"fmt"
	"log"
	// "net"
	"os"
)

const keyLength = 4096

var (
	info_log  *log.Logger
	error_log *log.Logger
)

var options struct {
	port      int
	host      string
	key       string
	publicKey string
	nick      string
}

func exit(status int, template string, args ...interface{}) {
	if status == 0 {
		info_log.Printf(template, args...)
	} else {
		error_log.Printf(template, args...)
	}
	os.Exit(status)
}

func usage(template string, args ...interface{}) {
	fmt.Fprintf(os.Stdout, template, args...)
	os.Exit(1)
}

func main() {
	flag.Parse()
	info_log, error_log = log.New(os.Stdout, "", 0), log.New(os.Stderr, "", 0)

	if flag.NArg() < 1 {
		usage("client or server?")
	}

	switch flag.Arg(0) {
	case "client":
		connect()
	case "server":
		serve()
	case "generate":
		generate()
	case "encrypt":
		encrypt()
	case "decrypt":
		decrypt()
	case "get-public":
		getPublic()
	default:
		usage("i dunno what you mean with %v", flag.Arg(0))
	}
}

func init() {
	flag.IntVar(&options.port, "port", 9000, "port number")
	flag.StringVar(&options.host, "host", "localhost", "host to connect to")
	flag.StringVar(&options.key, "key", "whisper_key", "rsa key to use")
	flag.StringVar(&options.publicKey, "public-key", "", "public rsa key to use")
	flag.StringVar(&options.nick, "nick", "", "nick to use in chat")
}
