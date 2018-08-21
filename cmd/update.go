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

// updateCmd represents the update command
var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Updates the project specified with information provided via command line args 'description', site-path', and 'local-path'.",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("update called")
		return nil
	},
}

func init() {
	updateCmd.Flags().String("localPath", "", "The string path for the updated local path for the project; checked by default (a non-existant path will not work), but can be forced.")
	updateCmd.Flags().String("sitePath", "", "The string path for the updated local path for the project.")
	updateCmd.Flags().StringP("description", "d", "", "Either updates a description manually if provided a string, otherwise the description will be fetched from the github page (In which case the '--token' arg will need to be setup).")
	updateCmd.Flags().StringP("token", "t", "", "Name of the json file in the project root with the gitlab token for gathering the project descriptions, or the token directly.")
	rootCmd.AddCommand(updateCmd)
}
