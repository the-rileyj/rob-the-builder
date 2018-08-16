// Copyright © 2018 NAME HERE <EMAIL ADDRESS>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var killCmd = &cobra.Command{
	Use:   "kill",
	Short: fmt.Sprintf("Kills the processes for %s (default) or kills all running ROB instances.", rjServer),
	RunE: func(cmd *cobra.Command, args []string) error {
		rob, err := cmd.Flags().GetBool("rob")

		if err != nil {
			return err
		}

		server, err := cmd.Flags().GetBool("server")

		if err != nil {
			return err
		}

		if rob && server {
			killClones()
			killServer()
		} else if rob {
			killClones()
		} else {
			killServer()
		}

		return nil
	},
}

func init() {
	killCmd.Flags().Bool("rob", false, "Kills all running instances of ROB.")
	killCmd.Flags().Bool("server", false, "Kills all running instances of RJserver.")
	rootCmd.AddCommand(killCmd)
}
