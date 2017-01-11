package server

import (
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/rpc"
	"github.com/gorilla/rpc/json"
)

type PunchService struct{}

func StartServer(args []string) error {
	fmt.Println("Server:", args)

	s := rpc.NewServer()
	s.RegisterCodec(json.NewCodec(), "application/json")
	s.RegisterService(new(PunchService), "P")
	http.Handle("/rpc", s)

	fmt.Println("I am listening!")
	log.Fatal(http.ListenAndServe(":8080", nil))
	return nil
}
