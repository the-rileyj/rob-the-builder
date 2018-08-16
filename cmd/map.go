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
	"strings"

	"github.com/spf13/cobra"
)

// mapCmd represents the map command
var mapCmd = &cobra.Command{
	Use:   "map",
	Short: "Path to a directory to visually map.",
	RunE: func(cmd *cobra.Command, args []string) error {
		spaces, err := cmd.Flags().GetUint64("spaces")

		if err != nil {
			return err
		}

		fmt.Println(newDirMap(strings.Join(args, " ")).ToString(spaces))

		return nil
	},
}

func init() {
	mapCmd.Flags().Uint64P("spaces", "s", 4, "Specifies the number of spaces for indendation.")
	// TODO: Add exclusion of directories
	// mapCmd.Flags().StringP("exclude", "e", 4, "Excludes mapping certain directories.")
	rootCmd.AddCommand(mapCmd)
}
