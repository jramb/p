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
	"os"

	"github.com/spf13/cobra"
)

var docCmd = &cobra.Command{
	Use:   "doc",
	Short: "documentation and tools for punch",
}

var autocompleteTarget string

// bashCmd represents the bash command
var bashCmd = &cobra.Command{
	Use:   "genautocomplete",
	Short: "Generate bash shell autocompletion for punch",
	Long: `Generates the autocompletion file for punch.

	sudo p doc genautocomplete

	add --completionfile=/path/to/file to set alternative file-path and name.
	use --completionfile=- to print to stdout.
	`,
	RunE: func(cmd *cobra.Command, args []string) error {
		//return RootCmd.GenBashCompletion(os.Stdout)
		if autocompleteTarget == "-" {
			return RootCmd.GenBashCompletion(os.Stdout)
		} else {
			return RootCmd.GenBashCompletionFile(autocompleteTarget)
		}
	},
}

func init() {
	RootCmd.AddCommand(docCmd)
	docCmd.AddCommand(bashCmd)
	//bashCmd.PersistentFlags().StringVarP(&autocompleteTarget, "completionfile	", "", "/etc/bash_completion.d/p", "Autocompletion file")
	bashCmd.PersistentFlags().StringVarP(&autocompleteTarget, "completionfile	", "", "/usr/share/bash-completion/completions/p", "Autocompletion file")

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// bashCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// bashCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")

}
