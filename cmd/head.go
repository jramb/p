// Copyright © 2016 Jörg Ramb <jorg@jramb.com>
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package cmd

import (
	"database/sql"
	"fmt"
	"github.com/jramb/p/tools"
	"github.com/spf13/cobra"
	"strings"
)

// headCmd represents the head command
var headCmd = &cobra.Command{
	Use:   "head",
	Short: "maintain headers",
	Long: `Functions to maintain the headers.
You need to have a header to create time entries.`,
}

var headListCmd = &cobra.Command{
	Use:   "list",
	Short: "Lists all active headers",
	Long:  `Lists all active headers.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return tools.WithOpenDB(true, func(db *sql.DB) error {
			return tools.ShowHeaders(db, args)
		})
	},
}

var headAddCmd = &cobra.Command{
	Use:   "add",
	Short: "add a new header",
	Long:  `Adds a new header.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return tools.WithTransaction(func(db *sql.DB, tx *sql.Tx) error {
			handle, args := tools.ParseHandle(args)
			if _, err := tools.VerifyHandle(db, handle, false); err == nil {
				return fmt.Errorf("handler '%s' does already exist", handle)
			}

			_, err := tools.AddHeader(tx, strings.Join(args, " "), handle)
			return err
		})
	},
}

func init() {
	RootCmd.AddCommand(headCmd)
	headCmd.AddCommand(headAddCmd)
	headCmd.AddCommand(headListCmd)
}
