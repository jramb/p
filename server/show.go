package server

import (
	"database/sql"
	"net/http"
	"time"

	"github.com/jramb/p/tools"
	"github.com/spf13/viper"
)

type ShowArgs struct {
	TimeFrame string
	Filter    string
}

type ShowReply struct {
	From                time.Time
	To                  time.Time
	TimeDurationEntries []tools.TimeDurationEntry
}

func (h *PunchService) Show(r *http.Request, args *ShowArgs, reply *ShowReply) error {
	error := tools.WithOpenDB(true, func(db *sql.DB) error {
		bias := viper.GetDuration("show.bias")
		rounding := viper.GetDuration("show.rounding")
		timeFrame := args.TimeFrame
		filter := args.Filter
		var err error
		reply.From, reply.To, err = tools.DecodeTimeFrame(timeFrame)
		if err != nil {
			panic(err)
		}
		if timeEntries, err := tools.QueryDays(db, reply.From, reply.To, filter, rounding, bias); err == nil {
			reply.TimeDurationEntries = timeEntries
		} else {
			return err
		}
		return nil
	})
	return error
}
