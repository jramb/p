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
	"github.com/jramb/p/tools"
	"github.com/spf13/cobra"
)

// inCmd represents the in command
var switchCmd = &cobra.Command{
	Use:   "switch",
	Short: "switch the currently running entry to another header",
	Long: `Continues the current entry to count for the given header instead.
Will otherwise not modify the currently running period. If none is active nothing happens.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return tools.WithTransaction(func(db *sql.DB, tx *sql.Tx) error {
			handle, args := tools.ParseHandle(args)
			handle, err := tools.VerifyHandle(db, handle, true)
			if err != nil {
				return err
			}
			effectiveTime := GetEffectiveTime()
			return tools.ChangeCheckIn(tx, args, handle, effectiveTime)
		})
	},
}

func init() {
	RootCmd.AddCommand(switchCmd)
}
