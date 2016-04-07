package main

import (
	"errors"
	"net/http"
	"test/routers"
	"time"
)

var (
	ServerIP   = "192.168.12.22"
	ServerPort = "12345"
	ListenPort = "9257"
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
	http.ListenAndServe(":"+ListenPort, router)

}
