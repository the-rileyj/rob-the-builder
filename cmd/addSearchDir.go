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
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

// addSearchDirCmd represents the addSearchDir command
var addSearchDirCmd = &cobra.Command{
	Use:   "searchDir",
	Short: "Adds a new search directory.",
	RunE: func(cmd *cobra.Command, args []string) error {
		searchDir := strings.TrimSpace(strings.Join(args, " "))

		_, err := os.Stat(searchDir)

		if err != nil {
			return errors.New("local path to searchDir does not exist")
		}

		searchDir, err = filepath.Abs(searchDir)

		if err != nil {
			return errors.New("could not get absolute path to project")
		}

		rjInfo, err := getRjInfo(projectRootPath)

		if err != nil {
			return err
		}

		for _, searchPath := range rjInfo.RJLocal.SearchPaths {
			if searchDir == searchPath {
				return errors.New("search path already in search path list")
			}
		}

		rjInfo.RJLocal.SearchPaths = append(rjInfo.RJLocal.SearchPaths, searchDir)

		return writeUpdate(projectRootPath, *rjInfo)
	},
}

func init() {
	addCmd.AddCommand(addSearchDirCmd)
}
