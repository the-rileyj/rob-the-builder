// Copyright © 2018 Riley Johnson
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
	"io/ioutil"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

// discoverCmd represents the discover command
var discoverCmd = &cobra.Command{
	Use:   "discover",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		force, err := cmd.Flags().GetBool("force")

		if err != nil {
			return err
		}

		rjInfo, err := getRjInfo(projectRootPath)

		if err != nil {
			return err
		}

		if len(rjInfo.RJLocal.SearchPaths) != 0 {
			discoveredTagPaths := make([]string, 0)

			for _, searchPath := range rjInfo.RJLocal.SearchPaths {
				discoveredTagPaths = append(discoveredTagPaths, fileSearcher(".RJtag", searchPath, -1)...)
			}

			fmt.Printf("Found %d Projects\n", len(discoveredTagPaths))

			errs, forced, found := 0, 0, 0

			for _, discoveredTagPath := range discoveredTagPaths {
				if id, err := ioutil.ReadFile(discoveredTagPath); err != nil {
					fmt.Println(errors.Wrapf(err, "could not get id from tag at %s", discoveredTagPath))
					errs++
				} else {
					_, localProjectExists := rjInfo.RJLocal.Projects[string(id)]
					if !localProjectExists {
						rjInfo.RJLocal.Projects[string(id)] = RJLocalProject{Path: filepath.Dir(discoveredTagPath)}
						found++
					} else if force {
						rjInfo.RJLocal.Projects[string(id)] = RJLocalProject{Path: filepath.Dir(discoveredTagPath)}
						forced++
					}
				}
			}

			pruned := pruneLocal(rjInfo)

			desc := "kept"
			result := found - pruned

			if force {
				desc = "forced"
				result = forced - pruned
			}

			if result < 0 {
				result = 0
			}

			fmt.Printf("%d Projects found and %s, %d pruned, and %d had errors occur reading the tags.\n", result, desc, pruned, errs)

			return writeUpdate(projectRootPath, *rjInfo)
		}

		return errors.New("cannot discover if the list of paths to search in is empty (RJlocal.SearchPaths)")
	},
}

func init() {
	discoverCmd.Flags().BoolP("force", "f", false, "TODO")
	rootCmd.AddCommand(discoverCmd)
}
