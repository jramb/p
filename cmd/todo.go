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
	"strings"
)

// todoCmd represents the todo command
var todoCmd = &cobra.Command{
	Use:   "todo",
	Short: "a collection of methods to maintain a TODO list",
	Long: `a collection of methods to maintain a context related TODO list.
TODO entries are usually related to the current header.`,
	//RunE: todoListCmd.RunE,
}

var todoListCmd = &cobra.Command{
	Use:   "list",
	Short: "list current TODOs",
	Long:  `Lists the current TODOs. This is context sensitive.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return tools.WithOpenDB(true, func(db *sql.DB) error {
			handle, args := tools.ParseHandle(args)
			handle, err := tools.VerifyHandle(db, handle, true)
			if err != nil {
				return err
			}
			return tools.ShowTodo(db, args, handle, 9999)
		})
	},
}

var todoAddCmd = &cobra.Command{
	Use:   "add",
	Short: "add a new TODO entry",
	Long:  `Adds a new TODO entry to the currently running task/header.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return tools.WithTransaction(func(db *sql.DB, tx *sql.Tx) error {
			handle, args := tools.ParseHandle(args)
			handle, err := tools.VerifyHandle(db, handle, true)
			if err != nil {
				return err
			}
			effectiveTime := GetEffectiveTime()
			return tools.AddTodo(tx, strings.Join(args, " "), handle, effectiveTime)
		})
	},
}

var todoDoneCmd = &cobra.Command{
	Use:   "done [nn {nn ...}]",
	Short: "mark TODO entries as done",
	Long:  `Marks the given TODO entry/entries as done`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return tools.WithTransaction(func(db *sql.DB, tx *sql.Tx) error {
			handle, args := tools.ParseHandle(args)
			handle, err := tools.VerifyHandle(db, handle, true)
			if err != nil {
				return err
			}
			effectiveTime := GetEffectiveTime()
			return tools.TodoDone(tx, args, handle, effectiveTime)
		})
	},
}

var todoUndoCmd = &cobra.Command{
	Use:   "undo",
	Short: "mark a TODO entry as not done (reverses 'done')",
	Long:  `Marks the given TODO entry again as not yet done`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return tools.WithTransaction(func(db *sql.DB, tx *sql.Tx) error {
			handle, args := tools.ParseHandle(args)
			handle, err := tools.VerifyHandle(db, handle, true)
			if err != nil {
				return err
			}
			return tools.TodoUndo(tx, args, handle)
		})
	},
}

func init() {
	RootCmd.AddCommand(todoCmd)
	todoCmd.AddCommand(todoAddCmd)
	todoCmd.AddCommand(todoListCmd)
	todoCmd.AddCommand(todoDoneCmd)
	todoCmd.AddCommand(todoUndoCmd)
}
