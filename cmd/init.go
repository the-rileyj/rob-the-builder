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
	"github.com/spf13/cobra"
)

// initCmd represents the init command
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new RJglobal file if not already created. Can be forced.",
	RunE: func(cmd *cobra.Command, args []string) error {
		force, err := cmd.Flags().GetBool("force")

		if err != nil {
			return err
		}

		local, err := cmd.Flags().GetBool("local")

		if err != nil {
			return err
		}

		if local {
			// RJlocal is autogen'd when it can't be found during information retrieval.
			_, err = getRjInfo(projectRootPath)
		} else {
			_, err = initializeGlobal(projectRootPath, force)
		}

		return err
	},
}

func init() {
	initCmd.Flags().BoolP("force", "f", false, "Force RJglobal/RJlocal file to be created in the root path specified.")
	initCmd.Flags().BoolP("local", "l", false, "Explicitely initialize RJlocal file in the root path specified.")
	rootCmd.AddCommand(initCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// initCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// initCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
