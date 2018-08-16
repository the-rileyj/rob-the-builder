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
	"os"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

// buildCmd represents the build command
var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "Builds either the local project specified or all local projects if no project is specified.",
	Long: `Builds either the local project specified or all local projects if no project is specified.
Will check the last build hash prior to building and if they are the same then the project will only be rebuilt if the '-force' flag is included.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		force, err := cmd.Flags().GetBool("force")

		if err != nil {
			return err
		}

		root, err := cmd.Flags().GetBool("root")

		if err != nil {
			return err
		}

		rjInfo, err := getRjInfo(projectRootPath)

		if err != nil {
			return err
		}

		if root {
			if _, err := os.Stat(projectRootPath); err != nil {
				return errors.Wrap(err, "path to project root does not exist")
			}

			localHash, err := getLocalProjectCommit(projectRootPath)

			if err != nil {
				return errors.Wrap(err, "problem fetching remote commit hash for root project")
			}

			if localHash != rjInfo.RJLocal.LastRemoteHashOnBuild || force {
				if remoteHash, err := getRemoteProjectCommit(rjInfo.RJGlobal.URL); err == nil && remoteHash != localHash {
					fmt.Println("Local project is not synced with remote, make sure to push/pull as needed.")
				}

				if err = buildRoot(projectRootPath); err != nil {
					return err
				}

				rjInfo.RJLocal.LastRemoteHashOnBuild = localHash
				return writeUpdate(projectRootPath, *rjInfo)
			}

			cmd.Println("Skipping building root project because the last build hash matches the local commit hash, please specify '-force' if you wish to override.")
			return nil
		}

		var rerr error

		project := strings.TrimSpace(strings.Join(args, " "))
		var needUpdate, update bool

		if project == "" {
			for _, rjProject := range rjInfo.RJGlobal.Projects {
				if needUpdate, err = rjBuild(rjInfo, rjProject, projectRootPath, force); needUpdate {
					update = true
				} else if err != nil {
					rerr = err
				}
			}

			if update {
				return writeUpdate(projectRootPath, *rjInfo)
			}

			return rerr
		}

		if index := getProjectIndex(project, rjInfo.RJGlobal.Projects); index != -1 {
			update, err = rjBuild(rjInfo, rjInfo.RJGlobal.Projects[index], projectRootPath, force)

			if update {
				return writeUpdate(projectRootPath, *rjInfo)
			}

			return err
		}

		return errors.New("project specified does not exist")
	},
}

func init() {
	buildCmd.Flags().BoolP("force", "f", false, "Forces the project to be built.")
	buildCmd.Flags().Bool("root", false, "Builds the webserver in the project root.")
	rootCmd.AddCommand(buildCmd)
}
