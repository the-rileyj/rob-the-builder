// Copyright © 2018 Riley Johnson rj@therileyjohnson.com
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

	"github.com/spf13/cobra"
)

const (
	reactLocalDockerfile  = "react-local-build.dockerfile"
	reactRemoteDockerfile = "react-remote-build.dockerfile"
	rjServer              = "RJserver"
	rjURL                 = "https://therileyjohnson.com"
)

var projectRootPath string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "cob",
	Short: "ROB is a build tool for React projects in RJ's site.",
	Long: `ROB is a build tool for React projects in RJ's site.

ROB helps organize React projects on a site level and a local level to allow for building both remotely and locally to the path relative to the site root.
Additionally, ROB is used for webserver lifecycle automation through running the server and handling updates via return codes indicating such.
There are other uses in addition to the list above, find them out by running 'rob'.`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.
	rootCmd.PersistentFlags().StringVarP(&projectRootPath, "projectRoot", "r", "./", "Path to the project root.")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	// rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
