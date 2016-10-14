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

// logCmd represents the ll command
var logCmd = &cobra.Command{
	Use:   "log",
	Short: "log related tools",
	Long:  `Several log tools`,
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "list log entries",
	Long:  `Shows the log entries`,
	RunE: func(cmd *cobra.Command, args []string) error {
		//defer tools.RollbackOnError(tx)
		//D(args)
		if db, err := tools.OpenDB(true); err == nil {
			defer db.Close()

			return tools.ListLogEntries(db, args)
		} else {
			return err
		}
	},
}

func init() {
	RootCmd.AddCommand(logCmd)
	logCmd.AddCommand(listCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// llCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// llCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")

}
