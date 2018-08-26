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

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

// removeCmd represents the remove command
var removeCmd = &cobra.Command{
	Aliases: []string{
		"rm",
	},
	Use:   "remove",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		local, err := cmd.Flags().GetBool("local")

		if err != nil {
			return err
		}

		rjInfo, err := getRjInfo(projectRootPath)

		if err != nil {
			return err
		}

		project := strings.TrimSpace(strings.Join(args, " "))

		if project == "" {
			return errors.New("project must be specified")
		}

		if local {
			removeProjectLocally(project, rjInfo)
			return writeUpdate(projectRootPath, *rjInfo)
		}

		// Remove globally
		index := getProjectIndex(project, rjInfo.RJGlobal.Projects)

		if index == -1 {
			errors.New("project specified does not exist")
		}

		projectName := rjInfo.RJGlobal.Projects[index].Name

		rjInfo.RJGlobal.Projects = append(rjInfo.RJGlobal.Projects[:index], rjInfo.RJGlobal.Projects[index+1:]...)

		// Pruning handles deleting project locally
		pruneLocal(rjInfo)

		fmt.Printf("Deleted Project %s Globally and Locally\n", projectName)

		return writeUpdate(projectRootPath, *rjInfo)
	},
}

func init() {
	removeCmd.Flags().BoolP("local", "l", false, "Deletes the specified project locally only.")
	rootCmd.AddCommand(removeCmd)
}
