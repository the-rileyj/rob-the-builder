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

// listCmd represents the list command
var listCmd = &cobra.Command{
	Aliases: []string{
		"ls",
	},
	Use:   "list",
	Short: "Lists the details of the project specified, or all project if one is not specified.",
	Long: `Lists the details of the project specified, or all project if one is not specified as pretty printed JSON.
Filters for global and local can be specified.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		global, err := cmd.Flags().GetBool("global")

		if err != nil {
			return err
		}

		local, err := cmd.Flags().GetBool("local")

		if err != nil {
			return err
		}

		spaces, err := cmd.Flags().GetUint64("spaces")

		if err != nil {
			return err
		}

		rjInfo, err := getRjInfo(projectRootPath)

		if err != nil {
			return err
		}

		project := strings.TrimSpace(strings.Join(args, " "))

		if (global && local) || (!global && !local) {
			if project == "" {
				index := getProjectIndex(project, rjInfo.RJGlobal.Projects)

				if index == -1 {
					return errors.New("project specified does not exist")
				}
				return printProject(rjInfo.RJGlobal.Projects[index], rjInfo.RJLocal, spaces)
			}

			var rerr error

			for _, rjProject := range rjInfo.RJGlobal.Projects {
				if err = printProject(rjProject, rjInfo.RJLocal, spaces); err != nil {
					rerr = err
				}
			}

			return rerr
		} else if global {
			if project == "" {

				index := getProjectIndex(project, rjInfo.RJGlobal.Projects)

				if index == -1 {
					return errors.New("project specified does not exist")
				}

				fmt.Printf("%s\n", strings.Repeat("=", len(rjInfo.RJGlobal.Projects[index].Name)))

				err = prettyPrintStruct(rjInfo.RJGlobal.Projects[index], spaces)

				fmt.Printf("%s\n", strings.Repeat("=", len(rjInfo.RJGlobal.Projects[index].Name)))

				return err
			}

			return prettyPrintStruct(rjInfo.RJGlobal, spaces)
		}

		// Local selected
		if project == "" {
			index := getProjectIndex(project, rjInfo.RJGlobal.Projects)

			if index == -1 {
				return errors.New("project specified does not exist")
			}

			localProject, localProjectExists := rjInfo.RJLocal.Projects[rjInfo.RJGlobal.Projects[index].ID]

			if !localProjectExists {
				return errors.New("project specified does not exist locally")
			}

			fmt.Printf("%s\n", strings.Repeat("=", len(rjInfo.RJGlobal.Projects[index].Name)))

			err = prettyPrintStruct(localProject, spaces)

			fmt.Printf("%s\n", strings.Repeat("=", len(rjInfo.RJGlobal.Projects[index].Name)))

			return err
		}

		return prettyPrintStruct(rjInfo.RJLocal, spaces)
	},
}

func init() {
	listCmd.Flags().BoolP("global", "g", false, "Filters the results to only show global configuration for project(s).")
	listCmd.Flags().BoolP("local", "l", false, "Filters the results to only show the local configuration for project(s).")
	listCmd.Flags().Uint64("spaces", 2, "Specifies the number of spaces for indendation.")
	rootCmd.AddCommand(listCmd)
}
