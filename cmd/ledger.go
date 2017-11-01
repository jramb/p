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
	"github.com/spf13/viper"
	"time"
)

// printCmd represents the print command
var ledgerCmd = &cobra.Command{
	Use:   "ledger", // ledger-cli time format
	Short: "time entry details in ledger-cli format",
	Long: `Prints the time entries in details,
one line per entry. The format is compatible with Ledger-cli.org `,
	RunE: func(cmd *cobra.Command, args []string) error {
		return tools.WithOpenDB(true, func(db *sql.DB) error {
			return tools.ShowLedger(db, args, viper.GetDuration("show.rounding"), viper.GetDuration("show.bias"))
		})
	},
}

func init() {
	RootCmd.AddCommand(ledgerCmd)

	ledgerCmd.PersistentFlags().DurationVarP(&RoundTime, "rounding", "", time.Minute, "round times according to this duration, e.g. 1m, 15m, 1h")
	ledgerCmd.PersistentFlags().DurationVarP(&RoundingBias, "bias", "", time.Duration(0), "rounding bias (duration, default 0, max 1/2 rounding.)")
	// ledgerCmd.PersistentFlags().BoolVarP(&ShowRounding, "display-rounding", "r", false, "display rounding difference in output")
	// ledgerCmd.PersistentFlags().StringVarP(&DurationStyle, "style", "", "hour", "show duration style: time / hour")

	viper.BindPFlag("show.rounding", ledgerCmd.PersistentFlags().Lookup("rounding"))
	// viper.BindPFlag("show.style", ledgerCmd.PersistentFlags().Lookup("style"))
	viper.BindPFlag("show.bias", ledgerCmd.PersistentFlags().Lookup("bias"))
	viper.BindPFlag("show.display-rounding", ledgerCmd.PersistentFlags().Lookup("display-rounding"))
}
