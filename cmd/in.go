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
	"github.com/jramb/p/tools"
	"github.com/spf13/cobra"
)

// inCmd represents the in command
var inCmd = &cobra.Command{
	Use:   "in",
	Short: "punch in a new entry (start a period)",
	Long: `Starts a new entry for the given project.
Also automatically ends the currently running period (if any is active).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if db, err := tools.OpenDB(true); err == nil {
			defer db.Close()
			tx, err := db.Begin() // was: tools GetTx
			if err != nil {
				return err
			}
			defer tools.RollbackOnError(tx)

			handle, args := tools.ParseHandle(args)
			handle, err = tools.VerifyHandle(db, handle, true)
			if err != nil {
				return err
			}
			if err := tools.CloseAll(tx); err != nil {
				return err
			}
			return tools.CheckIn(tx, args, handle)
		} else {
			return err
		}
	},
}

func init() {
	RootCmd.AddCommand(inCmd)
}
