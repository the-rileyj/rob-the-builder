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
	"strings"

	"github.com/spf13/cobra"
)

// pushCmd represents the push command
var pushCmd = &cobra.Command{
	Use:   "push",
	Short: "Builds the rob installer dockerfile and pushes it with the tag specified.",
	RunE: func(cmd *cobra.Command, args []string) error {
		local, err := cmd.Flags().GetBool("local")

		if err != nil {
			return err
		}

		tag := strings.TrimSpace(strings.Join(args, " "))

		return rjPushRob(tag, local)
	},
}

func init() {
	pushCmd.Flags().BoolP("local", "l", false, "Builds the image from a local context.")
	rootCmd.AddCommand(pushCmd)
}
