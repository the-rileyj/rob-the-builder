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

// pruneCmd represents the prune command
var pruneCmd = &cobra.Command{
	Use:   "prune",
	Short: "Deletes all local projects not found in RJglobal.",
	RunE: func(cmd *cobra.Command, args []string) error {
		rjInfo, err := getRjInfo(projectRootPath)

		if err != nil {
			return err
		}

		pruned := pruneLocal(rjInfo)

		if pruned == 0 {
			fmt.Println("No Local Projects Pruned.")
			return nil
		}

		fmt.Printf("%d Local Projects Pruned.\n", pruned)
		return writeUpdate(projectRootPath, *rjInfo)
	},
}

func init() {
	rootCmd.AddCommand(pruneCmd)
}
