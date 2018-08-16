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
	"strings"

	"github.com/spf13/cobra"
)

// syncCmd represents the sync command
var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Checks the local git hash against what's in the remote repo and updates either the local project specified by 'project' or all local projects if 'project' is not specified.",
	RunE: func(cmd *cobra.Command, args []string) error {
		rjInfo, err := getRjInfo(projectRootPath)

		if err != nil {
			return err
		}

		project := strings.TrimSpace(strings.Join(args, " "))

		if project == "" {
			for _, rjProject := range rjInfo.RJGlobal.Projects {
				if printString, err := handleSyncronizeLocal(&rjProject, &rjInfo.RJLocal); err != nil {
					cmd.Println(err)
				} else {
					cmd.Println(printString)
				}
			}
		} else {
			if index := getProjectIndex(project, rjInfo.RJGlobal.Projects); index != -1 {
				rjProject := rjInfo.RJGlobal.Projects[index]

				if printString, err := handleSyncronizeLocal(&rjProject, &rjInfo.RJLocal); err != nil {
					cmd.Println(err)
				} else {
					cmd.Println(printString)
				}
			} else {
				fmt.Println("Project specified does not exist.")
			}
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(syncCmd)
}
