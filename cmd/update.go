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
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

// updateCmd represents the update command
var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Updates the project specified with information provided via command line args 'description', site-path', and 'local-path'.",
	RunE: func(cmd *cobra.Command, args []string) error {
		description, err := cmd.Flags().GetString("description")

		if err != nil {
			return err
		}

		localPath, err := cmd.Flags().GetString("localPath")

		if err != nil {
			return err
		}

		sitePath, err := cmd.Flags().GetString("sitePath")

		if err != nil {
			return err
		}

		token, err := cmd.Flags().GetString("token")

		if err != nil {
			return err
		}

		if token == "" {
			if _, err := os.Stat(filepath.Join(projectRootPath, "token.json")); err == nil {
				token, err = getGithubToken(filepath.Join(projectRootPath, "token.json"))

				if err != nil {
					return errors.Wrap(err, "could not get token from token.json")
				}
			}
		} else if _, err := os.Stat(token); err == nil {
			token, err = getGithubToken(token)

			if err != nil {
				return errors.Wrap(err, "could not get token from token.json")
			}
		}

		project := strings.TrimSpace(strings.Join(args, " "))

		var update bool

		rjInfo, err := getRjInfo(projectRootPath)

		if err != nil {
			return err
		}

		if project != "" {
			index := getProjectIndex(project, rjInfo.RJGlobal.Projects)

			if index == -1 {
				return errors.New("specified project does not exist")
			}

			rjProject := rjInfo.RJGlobal.Projects[index]

			if description != "" {
				rjProject.Description = description

				rjInfo.RJGlobal.Projects[index] = rjProject

				update = true
			} else if token != "" {
				newDescription, err := getProjectDescription(rjProject.Name, token)

				if err != nil {
					return err
				}

				if newDescription == rjProject.Description {
					return errors.New("new description is the same as the old one")
				}

				rjProject.Description = newDescription

				update = true
			}

			if localPath != "" {
				rjLocalProject, localProjectExists := rjInfo.RJLocal.Projects[rjProject.ID]

				if !localProjectExists {
					return fmt.Errorf("%s does not exist locally", rjProject.Name)
				}

				_, err := os.Stat(localPath)

				if err != nil {
					return errors.New("local path to project does not exist")
				}

				localPath, err = filepath.Abs(localPath)

				if err != nil {
					return errors.New("could not get absolute path to project")
				}

				if rjLocalProject.Path == localPath {
					return errors.New("the updated path is the same as the old path")
				}

				if rjLocalProject.Path != "" {
					if _, err := os.Stat(filepath.Join(rjLocalProject.Path, ".RJtag")); err == nil {
						os.Rename(filepath.Join(rjLocalProject.Path, ".RJtag"), filepath.Join(localPath, ".RJtag"))
					}
				} else {
					if err = writeRjTag(rjProject.ID, localPath); err != nil {
						return errors.Wrap(err, "problem automatically writing to the .RJtag file in the new project's local path")
					}
				}

				rjLocalProject.Path = localPath

				rjInfo.RJLocal.Projects[rjProject.ID] = rjLocalProject

				update = true
			}

			if sitePath != "" {
				rjProject.SitePath = sitePath

				rjInfo.RJGlobal.Projects[index] = rjProject

				update = true
			}

			if update {
				return writeUpdate(projectRootPath, *rjInfo)
			}

			return nil
		}

		if token != "" {
			var newDescription string

			for index, rjProject := range rjInfo.RJGlobal.Projects {

				newDescription, err = getProjectDescription(rjProject.Name, token)

				if err != nil {
					cmd.Println(errors.Wrapf(err, "problem fetching description for Project '%s'", rjProject.Name))
				} else {
					if newDescription != rjProject.Description {
						rjInfo.RJGlobal.Projects[index].Description = newDescription
						cmd.Printf("Description for Project '%s' successfully updated.", rjProject.Name)
						update = true
					}
				}
			}

			if !update {
				return errors.New("descriptions for all projects are already up to date")
			}

			return writeUpdate(projectRootPath, *rjInfo)
		}

		for _, rjProject := range rjInfo.RJGlobal.Projects {
			created := 0
			if _, rjLocalExists := rjInfo.RJLocal.Projects[rjProject.ID]; !rjLocalExists {
				rjInfo.RJLocal.Projects[rjProject.ID] = RJLocalProject{}
				created++
			}

			if created != 0 {
				cmd.Printf("%d Projects created from RJglobal to RJlocal!", created)
				update = true
			} else {
				cmd.Println("Local Projects already in sync with global Projects.")
			}
		}

		return nil
	},
}

func init() {
	updateCmd.Flags().StringP("description", "d", "", "Either updates a description manually if provided a string, otherwise the description will be fetched from the github page (In which case the '--token' arg will need to be required).")
	updateCmd.Flags().String("localPath", "", "The string path for the updated local path for the project; checked by default (a non-existant path will not work), but can be forced.")
	updateCmd.Flags().String("sitePath", "", "The string path for the updated local path for the project.")
	updateCmd.Flags().StringP("token", "t", "", "Name of the json file in the project root with the gitlab token for gathering the project descriptions, or the token directly.")
	rootCmd.AddCommand(updateCmd)
}
