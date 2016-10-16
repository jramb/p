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
	"github.com/spf13/viper"
	"time"
)

// showCmd represents the show command
var showCmd = &cobra.Command{
	Use:   "show", // aka "sum"
	Short: "shows the time entries in various formats",
	Long: `Shows the time entries in various different formats.
	
Most commands take an additional time-frame parameter:
week        = current week
month       = current month
day         = current day
today
yesterday

and optional a modifier:
week-2      = last week
month+1     = next month (probably empty)
today-1     = yesterday
`,
}

var showSumCmd = &cobra.Command{
	Use:   "sum", // aka "sum"
	Short: "show the time entries summarized",
	Long:  `Summarizes the times over a period of time. Shows the sum for each header.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return tools.WithOpenDB(true, func(db *sql.DB) error {
			return tools.ShowTimes(db, args, viper.GetDuration("show.rounding"), viper.GetDuration("show.bias"))
		})
	},
}

var showDaysCmd = &cobra.Command{
	Use:   "days",
	Short: "daily time summary for a period of time",
	Long:  `Shows the time entries, summarized on day basis.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return tools.WithOpenDB(true, func(db *sql.DB) error {
			bias := viper.GetDuration("show.bias")
			if err := tools.ShowDays(db, args, viper.GetDuration("show.rounding"), bias); err != nil {
				return err
			}
			//tools.Running(db, args, "", GetEffectiveTime())
			fmt.Println("=================================")
			return tools.ShowTimes(db, args, viper.GetDuration("show.rounding"), bias)
		})
	},
}

var RoundTime time.Duration
var RoundingBias time.Duration // rounding modifier should not be more then half the RoundTime
var ShowRounding bool

func init() {
	RootCmd.AddCommand(showCmd)
	showCmd.AddCommand(showSumCmd)
	showCmd.AddCommand(showDaysCmd)

	showCmd.PersistentFlags().DurationVarP(&RoundTime, "rounding", "", time.Minute, "round times according to this duration, e.g. 1m, 15m, 1h")
	showCmd.PersistentFlags().DurationVarP(&RoundingBias, "bias", "", time.Duration(0), "rounding bias (duration, default 0, max 1/2 rounding.)")
	showCmd.PersistentFlags().BoolVarP(&ShowRounding, "display-rounding", "r", false, "display rounding difference in output")

	viper.BindPFlag("show.rounding", showCmd.PersistentFlags().Lookup("rounding"))
	viper.BindPFlag("show.bias", showCmd.PersistentFlags().Lookup("bias"))
	viper.BindPFlag("show.display-rounding", showCmd.PersistentFlags().Lookup("display-rounding"))
}
