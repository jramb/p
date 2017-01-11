package server

import (
	"net/http"
)

type HelloArgs struct {
	Who string
}

type HelloReply struct {
	Message string
}

func (h *PunchService) Hello(r *http.Request, args *HelloArgs, reply *HelloReply) error {
	reply.Message = "Hello, " + args.Who + "!"
	return nil
}

//func init() {
//}
