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
	"strconv"

	"github.com/spf13/cobra"
)

// removeSearchDirCmd represents the removeSearchDir command
var removeSearchDirCmd = &cobra.Command{
	Use:   "searchDir",
	Short: "Removes a search directory.",
	RunE: func(cmd *cobra.Command, args []string) error {
		rjInfo, err := getRjInfo(projectRootPath)

		if err != nil {
			return err
		}

		var update bool

		for _, parseString := range args {
			index, err := strconv.ParseUint(parseString, 10, 64)

			if err == nil {
				if 0 <= int(index) || int(index) < len(rjInfo.RJLocal.SearchPaths) {
					rjInfo.RJLocal.SearchPaths = append(rjInfo.RJLocal.SearchPaths[:index], rjInfo.RJLocal.SearchPaths[index+1:]...)
					update = true
				}
			}
		}

		if update {
			return writeUpdate(projectRootPath, *rjInfo)
		}

		return nil
	},
}

func init() {
	removeCmd.AddCommand(removeSearchDirCmd)
}
