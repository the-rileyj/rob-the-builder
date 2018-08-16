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
	"strings"
	"sync"
)

//==================
// Local Use Structs

// RJInfo is for centralizing global and local information for the program's use
type RJInfo struct {
	RJGlobal
	RJLocal
	token string
}

// RJGlobal is for storing global information about projects and the project root URL, committed
type RJGlobal struct {
	Projects []RJProject `json:"projects"`
	URL      string      `json:"url"`
}

// RJLocalProject is for storing local information about a given project, not committed
type RJLocalProject struct {
	Path            string // Used when building from local
	LastBuildCommit string // Used when building from remote
	LastBuildHash   string // Used when building from local
}

// RJProject is for storing global information about a given project, committed
type RJProject struct {
	Description string `json:"description"`
	ID          string `json:"id"`
	Name        string `json:"name"`
	SitePath    string `json:"sitePath"`
	URL         string `json:"url"`
}

// RJLocal is for storing local information about projects, the last commit hash of the webserver, and where to start searching for local projects, not committed
type RJLocal struct {
	Projects              map[string]RJLocalProject `json:"projects"`
	SearchPaths           []string                  `json:"searchPaths"`
	LastRemoteHashOnBuild string                    `json:"lastRemoteHashOnBuild"`
}

type arguments struct {
	add, build, clone, discover, flightCheck, force, initialize, initializeLocal, kill, list, local, prune, syncronizeLocal, remove, run, root, suicide, update, upgrade, updateDescription bool
	spaces                                                                                                                                                                                  uint64
	selectProject, mapDirectory, projectRoot, pushat, token, tokenFilePath, updateSearchPath, updateSitePath, updateLocalPath                                                               string
}

type githubToken struct {
	Token string `json:"token"`
}

//===========================================================================
// Signal for sending success signal to processes, satisfies Signal interface

// RJSignal struct for indicating that shell process finished properly and the main process can stop listening for CTRL+C so it can kill that process */
type RJSignal struct{}

func (rjs RJSignal) String() string {
	return "RJ"
}

// Signal satisfies the os.Signal interface
func (rjs RJSignal) Signal() {}

//===============================
// For use in mapping directories

type dirMap struct {
	Dirs             []dirMap
	Dir, DirName     string
	FromRoot, Weight uint64
}

func (d dirMap) Len() int {
	return len(d.Dirs)
}

func (d dirMap) Less(i, j int) bool {
	return d.Dirs[i].Weight < d.Dirs[j].Weight
}

func (d dirMap) Swap(i, j int) {
	d.Dirs[i], d.Dirs[j] = d.Dirs[j], d.Dirs[i]
}

func (d dirMap) String() string {
	return d.ToString(4)
}

func (d dirMap) ToString(spaces uint64) string {
	var previous string
	var dm dirMap
	var dmi dirMapStackItem
	var last bool
	rString := strings.Builder{}
	dirMapStack := newDirMapStack()
	rString.WriteString("+" + d.DirName + "\n")

	for index, sd := range d.Dirs {
		dirMapStack.Push(dirMapStackItem{sd, "", index == 0})
	}

	for len(dirMapStack.items) != 0 {
		dmi = dirMapStack.Pop()

		dm = dmi.dirMap
		previous = dmi.previous
		last = dmi.last

		rString.WriteString(previous)
		rString.WriteString("|\n")
		rString.WriteString(previous)
		rString.WriteString("o")
		rString.WriteString(strings.Repeat("-", int(spaces)-1))
		rString.WriteString(">/")
		rString.WriteString(dm.DirName)
		rString.WriteString("\n")

		if last {
			for index, sd := range dm.Dirs {
				dirMapStack.Push(dirMapStackItem{sd, previous + strings.Repeat(" ", int(spaces)), index == 0})
			}
		} else {
			for index, sd := range dm.Dirs {
				dirMapStack.Push(dirMapStackItem{sd, previous + "|" + strings.Repeat(" ", int(spaces)-1), index == 0})
			}
		}
	}

	return rString.String()
}

// func (d dirMap) RootString(previous string, spaces uint64) string {
// 	rString := strings.Builder{}

// 	rString.WriteString("+" + d.DirName + "\n")
// 	subDirLength := len(d.Dirs)
// 	for index, sd := range d.Dirs {
// 		rString.WriteString(sd.SubRootString(index+1 == subDirLength, "", spaces))
// 	}

// 	return rString.String()
// }

// func (d dirMap) SubRootString(last bool, previous string, spaces uint64) string {
// 	rString := strings.Builder{}

// 	rString.WriteString(previous)
// 	rString.WriteString("|\n")
// 	rString.WriteString(previous)
// 	rString.WriteString("o")
// 	rString.WriteString(strings.Repeat("-", int(spaces)-1))
// 	rString.WriteString(">/")
// 	rString.WriteString(d.DirName)
// 	rString.WriteString("\n")

// 	subDirLength := len(d.Dirs)
// 	if last {
// 		for index, sd := range d.Dirs {
// 			rString.WriteString(sd.SubRootString(index+1 == subDirLength, previous+strings.Repeat(" ", int(spaces)), spaces))
// 		}
// 	} else {
// 		for index, sd := range d.Dirs {
// 			rString.WriteString(sd.SubRootString(index+1 == subDirLength, previous+"|"+strings.Repeat(" ", int(spaces)-1), spaces))
// 		}
// 	}

// 	return rString.String()
// }

//============================
// Request Use Data Structures

// GraphQL Query to get the description for a project
var descriptionQuery = `
query {
	repository(owner:"the-rileyj", name:"%s") {
		description
	}
}`

type data struct {
	Repository repository `json:"repository"`
}

type query struct {
	Query string `json:"query"`
}

type repository struct {
	Description string `json:"description"`
}

type response struct {
	Data data `json:"data"`
}

//============================
// NPM package.json data structure

type npmPackage struct {
	Name            string          `json:"name,omitempty"`
	Version         string          `json:"version,omitempty"`
	Private         bool            `json:"private,omitempty"`
	Homepage        string          `json:"homePage,omitempty"`
	Dependencies    dependencies    `json:"dependencies"`
	Scripts         scripts         `json:"scripts"`
	DevDependencies devDependencies `json:"devDependencies"`
}

type dependencies map[string]string

type devDependencies map[string]string

type scripts map[string]string

type dirMapStackItem struct {
	dirMap
	previous string
	last     bool
}

type dirMapStack struct {
	items []dirMapStackItem
	lock  sync.RWMutex
}

// Creates a new Stack
func newDirMapStack() *dirMapStack {
	return &dirMapStack{items: make([]dirMapStackItem, 0)}
}

// Push adds an Item to the top of the stack
func (s *dirMapStack) Push(t dirMapStackItem) {
	s.lock.Lock()
	s.items = append(s.items, t)
	s.lock.Unlock()
}

// Pop removes an Item from the top of the stack, panics if stack is empty
func (s *dirMapStack) Pop() dirMapStackItem {
	if len(s.items) == 0 {
		panic("stack is empty")
	}

	s.lock.Lock()
	item := s.items[len(s.items)-1]
	s.items = s.items[0 : len(s.items)-1]
	s.lock.Unlock()
	return item
}

type stringStack struct {
	items []string
	lock  sync.RWMutex
}

// Creates a new Stack
func newStringStack() *stringStack {
	return &stringStack{items: make([]string, 0)}
}

// Push adds an Item to the top of the stack
func (s *stringStack) Push(t string) {
	s.lock.Lock()
	s.items = append(s.items, t)
	s.lock.Unlock()
}

// Pop removes an Item from the top of the stack, panics if stack is empty
func (s *stringStack) Pop() string {
	if len(s.items) == 0 {
		panic("stack is empty")
	}

	s.lock.Lock()
	item := s.items[len(s.items)-1]
	s.items = s.items[0 : len(s.items)-1]
	s.lock.Unlock()
	return item
}
