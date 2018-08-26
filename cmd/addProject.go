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
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

// addProjectCmd represents the project command
var addProjectCmd = &cobra.Command{
	Use:   "project",
	Short: "Adds a project to RJglobal and RJlocal.",
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

		projectURL := strings.TrimSpace(strings.Join(args, " "))

		if projectURL == "" {
			return errors.New("need project URL to add project")
		}

		if _, err := url.ParseRequestURI(projectURL); err != nil {
			return errors.New("Project URL is not valid")
		}

		rjInfo, err := getRjInfo(projectRootPath)

		if err != nil {
			return err
		}

		if checkProjectExistance(projectURL, rjInfo.RJGlobal.Projects) {
			return errors.New("project with that URL already exists")
		}

		projectID := generateID()

		if sitePath == "" {
			sitePath = filepath.Join("./projects/", path.Base(projectURL))
		}

		rjProject := RJProject{
			ID:       projectID,
			Name:     path.Base(projectURL),
			SitePath: sitePath,
			URL:      projectURL,
		}

		os.MkdirAll(rjProject.SitePath, os.ModePerm)

		if token != "" {
			rjProject.Description, err = getProjectDescription(rjProject.Name, rjInfo.token)

			if err != nil {
				cmd.Println("Could not fetch description, defaulting to blank.")
				rjProject.Description = ""
			}
		} else if description != "" {
			rjProject.Description = description
		}

		rjInfo.RJGlobal.Projects = append(rjInfo.RJGlobal.Projects, rjProject)
		rjInfo.RJLocal.Projects[projectID] = RJLocalProject{Path: localPath}

		if localPath != "" {
			if _, err := os.Stat(localPath); err == nil {
				if localPath, err = filepath.Abs(localPath); err == nil {
					if err = writeRjTag(projectID, localPath); err != nil {
						cmd.Println(errors.Wrap(err, "problem automatically writing to the .RJtag file in the new project's local path"))
					}
				} else {
					cmd.Println("could not get absolute path to project")
				}
			} else {
				cmd.Println("local path provided does not exist")
			}
		}

		return writeUpdate(projectRootPath, *rjInfo)
	},
}

func init() {
	addProjectCmd.Flags().StringP("description", "d", "", "Either updates a description manually if provided a string, otherwise the description will be fetched from the github page (In which case the '--token' arg will need to be required).")
	addProjectCmd.Flags().String("localPath", "", "The string path for the local path for the project; checked by default (a non-existant path will not work), but can be forced.")
	addProjectCmd.Flags().String("sitePath", "", "The string path for the site path for the project.")
	addProjectCmd.Flags().StringP("token", "t", "", "Name of the json file in the project root with the gitlab token for gathering the project descriptions, or the token directly.")
	addCmd.AddCommand(addProjectCmd)
}
