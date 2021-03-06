// Copyright © 2016 Jörg Ramb <jorg@jramb.com>

package cmd

import (
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"net/http"

	"github.com/gorilla/rpc/json"
	"github.com/jramb/p/tools"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type SyncArgs struct {
	Owner    string              `json:"owner"`
	Revision int                 `json:"revision"`
	Key      string              `json:"key"`
	Headers  *[]tools.JSONHeader `json:"headers"`
	Entries  *[]tools.JSONEntry  `json:"entries"`
}

type SyncReply struct {
	Owner    string             `json:"owner"`
	Revision int                `json:"revision"`
	Headers  []tools.JSONHeader `json:"headers"`
	Entries  []tools.JSONEntry  `json:"entries"`
}

func contactTimeServer(args SyncArgs) (*SyncReply, error) {
	rpcURL := viper.GetString("timeserver.rpcurl")
	if rpcURL == "" {
		return nil, errors.New("Time server not configured (timeserver.rpcurl)")
	}
	message, err := json.EncodeClientRequest("T.Sync", &args)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", rpcURL, bytes.NewBuffer(message))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	client := new(http.Client)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result SyncReply
	err = json.DecodeClientResponse(resp.Body, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func performSync(db *sql.DB, tx *sql.Tx) error {
	args := SyncArgs{
		Owner:    viper.GetString("timeserver.owner"),
		Key:      viper.GetString("timeserver.key"),
		Revision: tools.GetParamInt(tx, "revision", 0),
	}
	args.Headers, args.Entries = tools.GetUncommitted(tx)

	reply, err := contactTimeServer(args)
	if err != nil {
		return err
	}

	if reply.Revision > 0 {
		if err := tools.ApplyUpdates(tx, reply.Headers, reply.Entries, reply.Revision); err != nil {
			return err
		}

		if err := tools.CommitRevision(tx, reply.Revision); err != nil {
			return err
		}

		tools.SetParamInt(tx, "revision", reply.Revision)
	}

	fmt.Printf("Synced revision %d, push %d/%d, fetched %d/%d\n",
		reply.Revision, len(*args.Headers), len(*args.Entries),
		len(reply.Headers), len(reply.Entries))
	return nil
}

// syncCmd represents the show command
var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "sync with punch time server",
	Long:  `Currently this is internal/test functionality`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return tools.WithTransaction(performSync)
	},
}

func init() {
	RootCmd.AddCommand(syncCmd)
}
