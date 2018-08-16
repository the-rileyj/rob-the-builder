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

// cloneCmd represents the clone command
var cloneCmd = &cobra.Command{
	Use:   "clone",
	Short: "Clones the local project specified.",
	Long: `Clones the local project specified. If the local project does not have a local hash (or all local projects which do not have a local hash if no project is specified) it will be cloned to.
NOTE: this will clear the whole directory prior to cloning.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		force, err := cmd.Flags().GetBool("force")

		if err != nil {
			return err
		}

		rjInfo, err := getRjInfo(projectRootPath)

		if err != nil {
			return err
		}

		project := strings.TrimSpace(strings.Join(args, " "))

		if project == "" {
			var rerr error

			for _, rjProject := range rjInfo.RJGlobal.Projects {
				if err := handleCloneProject(&rjProject, &rjInfo.RJLocal, force); err != nil {
					cmd.Println(err)
					rerr = err
				}
			}

			return rerr
		}

		index := getProjectIndex(project, rjInfo.RJGlobal.Projects)

		if index == -1 {
			return fmt.Errorf("project specified does not exist")
		}

		rjProject := rjInfo.RJGlobal.Projects[index]

		return handleCloneProject(&rjProject, &rjInfo.RJLocal, force)
	},
}

func init() {
	cloneCmd.Flags().BoolP("force", "f", false, "Forces cloning for the project specified.")
	rootCmd.AddCommand(cloneCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// cloneCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// cloneCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
