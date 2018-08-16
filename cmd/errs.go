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
)

type errRjFileNotFound struct {
	RjGlobal, RjLocal bool
}

func (err errRjFileNotFound) Error() string {
	if err.RjGlobal && err.RjLocal {
		return fmt.Sprintf("Could not find either a RJglobal or RJlocal file at %s\n", projectRootPath)
	} else if err.RjGlobal {
		return fmt.Sprintf("Could not find RJglobal file at %s\n", projectRootPath)
	}
	return fmt.Sprintf("Could not find RJlocal file at %s\n", projectRootPath)
}

func newErrRjFileNotFound(rjGlobal, rjLocal bool) error {
	return &errRjFileNotFound{rjGlobal, rjLocal}
}
