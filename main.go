package main

import (
	"fmt"
	"net/http"
	"test/routers"
)

func main() {
	router := routers.NewRouter()
	http.ListenAndServe(":8000", router)
	fmt.Printf("listen on 8000")

}
