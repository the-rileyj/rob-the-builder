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
	"errors"
	"os"
	"path"

	"github.com/spf13/cobra"
)

// checkCmd represents the check command
var checkCmd = &cobra.Command{
	Use:  "check",
	Long: "Prints the output of the 'pre-flight' check, tells if 'RJglobal' and 'RJlocal' files are found.",
	RunE: func(cmd *cobra.Command, args []string) error {
		if _, err := os.Stat(projectRootPath); err != nil {
			return errors.New("Path to project root does not exist")
		}

		_, err := os.Stat(path.Join(projectRootPath, "RJglobal.json"))
		cmd.Printf("RJglobal file exists: %v\n", err == nil)

		_, err = os.Stat(path.Join(projectRootPath, "RJlocal.json"))
		cmd.Printf("RJlocal file exists:  %v\n", err == nil)

		return nil
	},
}

func init() {
	rootCmd.AddCommand(checkCmd)
}
