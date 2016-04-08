package main

import (
	"errors"
	"flag"
	"net/http"
	"test/routers"
	"time"

	"github.com/vmware/harbor/log"
)

var (
	ServerIP   string
	ServerPort string
	ListenPort string
)

func register(ip string, port string) error {
	client := new(BaseClient)
	client.Opts = new(ClientOpts)
	client.Opts.Url = "http://" + ip + ":" + port
	client.Opts.Timeout = time.Duration(10 * time.Second)

	resp, err := client.DoAction("/register/"+ListenPort, Get)

	defer func() {
		if resp != nil {
			resp.Body.Close()
		}
	}()
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		err = errors.New("register fail")
	}
	return err
}

func main() {
	go func() {
		err := register(ServerIP, ServerPort)
		if err != nil {
			panic(err)
		}
	}()
	router := routers.NewRouter()
	log.Info("listening on " + ListenPort)
	http.ListenAndServe(":"+ListenPort, router)

}

func init() {
	flag.StringVar(&ServerIP, "sip", "", "server ip")
	flag.StringVar(&ServerPort, "sport", "", "server port")
	flag.StringVar(&ListenPort, "lport", "", "listen port")

	flag.Parse()

	if len(ServerIP) == 0 || len(ServerPort) == 0 || len(ListenPort) == 0 {
		panic("invalid argument")
	}
}
