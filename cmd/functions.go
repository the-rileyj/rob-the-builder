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
	"bytes"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/pkg/errors"
	git "gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/storage/memory"
)

func buildProject(localPath, rootPath, sitePath, githubURL string, remote bool) (string, error) {
	newHash := ""

	if !remote {
		newHash = dasher(localPath, -1)
	}

	absRoot, err := filepath.Abs(rootPath)

	if err != nil {
		return "", err
	}

	var imageBuildArgs []string
	if remote {
		// Equivalent to:
		// docker build -t rjtest:latest \
		// --build-arg GITHUB_DIR={Project Name} --build-arg GITHUB_URL={Project Github URL} \
		// -f - {path to build context}
		imageBuildArgs = []string{
			"build", "-t", "rj-react-build:latest",
			"--build-arg", fmt.Sprintf("GITHUB_DIR=%s", path.Base(githubURL)),
			"--build-arg", fmt.Sprintf("GITHUB_URL=%s", githubURL),
			"-f", "-", filepath.Clean(localPath),
		}
	} else {
		// Equivalent to "docker build -t rj-react-build:latest -f - {path to build context}"
		imageBuildArgs = []string{
			"build", "-t", "rj-react-build:latest",
			"-f", "-",
			filepath.Clean(localPath),
		}
	}

	cmd := exec.Command("docker", imageBuildArgs...)

	if remote {
		cmd.Stdin = bytes.NewBufferString(remoteReactBuild)
	} else {
		cmd.Stdin = bytes.NewBufferString(localReactBuild)
	}

	cmd.Stdout = os.Stdout

	killChannel := make(chan os.Signal, 1)

	signal.Notify(killChannel,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT,
	)

	err = cmd.Start()

	if err != nil {
		return "", err
	}

	go manageProcessReaping(cmd, killChannel)

	err = cmd.Wait()

	// Indicates that 'manageProcessReaping' can exit
	killChannel <- RJSignal{}

	if err != nil {
		return "", err
	}

	dockerBuildName := generateID()

	// Equivalent to "run --rm -v "{local RJsite path}/{project site path}:/app/build" rj-react-build:latest"
	runBuildArgs := []string{
		"run", "--rm",
		"-v", fmt.Sprintf(`%s:/app/build`, filepath.Clean(path.Join(absRoot, sitePath))),
		"--name", dockerBuildName,
		"rj-react-build:latest",
	}

	cmd = exec.Command("docker", runBuildArgs...)

	cmd.Stdout = os.Stdout

	err = cmd.Start()

	if err != nil {
		return "", err
	}

	go manageRunReaping(dockerBuildName, killChannel)

	err = cmd.Wait()

	// Indicates that 'manageProcessReaping' can exit
	killChannel <- RJSignal{}

	return newHash, err
}

func buildProjectLocally(localPath, rootPath, sitePath string) (string, error) {
	return buildProject(localPath, rootPath, sitePath, "", false)
}

func buildProjectRemotely(rootPath, sitePath, githubURL string) error {
	_, err := buildProject("", rootPath, sitePath, githubURL, true)
	return err
}

// buildRoot builds the webserver in a docker container and outputs it to the project root
func buildRoot(rootPath string) error {
	buildName, goArch, goOS := rjServer, runtime.GOARCH, runtime.GOOS

	if goOS == "windows" {
		buildName += ".exe"
	}

	// Equivalent to "build -t rj-root-build:latest --build-arg GOOS=windows --build-arg GOARCH=amd64 --build-arg BUILD_NAME=rjtest -f -""
	runRootBuildArgs := []string{
		"build", "-t", "rj-root-build:latest",
		"--build-arg", fmt.Sprintf("BUILD_NAME=%s", buildName),
		"--build-arg", fmt.Sprintf("GOARCH=%s", goArch),
		"--build-arg", fmt.Sprintf("GOOS=%s", goOS),
		"-f", "-", rootPath,
	}

	cmd := exec.Command("docker", runRootBuildArgs...)

	cmd.Stdin = bytes.NewBufferString(rootBuild)

	cmd.Stdout = os.Stdout

	killChannel := make(chan os.Signal, 1)

	signal.Notify(killChannel,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT,
	)

	err := cmd.Start()

	if err != nil {
		return err
	}

	go manageProcessReaping(cmd, killChannel)

	err = cmd.Wait()

	// Indicates that 'manageProcessReaping' can exit
	killChannel <- RJSignal{}

	if err != nil {
		return err
	}

	dockerBuildName := generateID()

	//Equivalent to "run --rm rj-root-build:latest"
	runRootTransferArgs := []string{
		"run", "--rm", "--name", dockerBuildName, "rj-root-build:latest",
	}

	cmd = exec.Command("docker", runRootTransferArgs...)

	serverExecutable, err := os.Create(buildName)

	if err != nil {
		return err
	}

	cmd.Stdout = serverExecutable

	err = cmd.Start()

	if err != nil {
		return err
	}

	go manageRunReaping(dockerBuildName, killChannel)

	err = cmd.Wait()

	// Indicates that 'manageProcessReaping' can exit
	killChannel <- RJSignal{}

	return err
}

// Checks the old build hash against the current hash of the directly; the output hash is not always consistant,
// so a hash is generated five times and compared against the old hash, returning true if any of the five match
func checkProjectBuildHash(oldHash, projectPath string) bool {
	for iteration := 0; iteration < 133; iteration++ {
		if oldHash == dasher(projectPath, -1) {
			return true
		}
	}
	return false
}

// checkProjectExistance checks if the project URL provided already exists (true)
func checkProjectExistance(identifier string, projects []RJProject) bool {
	for _, project := range projects {
		if project.URL == identifier {
			return true
		}
	}
	return false
}

func cloneProject(rjLocalProjectPath, rjProjectURL string) error {
	if err := removeContents(rjLocalProjectPath); err != nil {
		return err
	}

	_, err := git.PlainClone(rjLocalProjectPath, false, &git.CloneOptions{
		URL:               rjProjectURL,
		RecurseSubmodules: git.DefaultSubmoduleRecursionDepth,
	})
	return err
}

func dasher(rootPath string, maxChanNumber int) string {
	type directoryHasher struct {
		directoryPaths []string
		fileHashes     [][20]byte
	}

	if maxChanNumber < 1 {
		maxChanNumber = runtime.GOMAXPROCS(0)
	}

	currentChanNumber := 0
	pathChan := make(chan directoryHasher, maxChanNumber)
	checkPaths := []string{rootPath}
	hasher := sha1.New()

	getFileHashes := func(dirname string) {
		if f, err := os.Open(dirname); err != nil {
			pathChan <- directoryHasher{}
		} else {
			foundPaths, err := f.Readdir(-1)
			f.Close()
			if err != nil {
				pathChan <- directoryHasher{}
			} else {
				returnDirectorySearched := directoryHasher{make([]string, 0), make([][20]byte, 0)}
				for _, foundPath := range foundPaths {
					if foundPath.IsDir() {
						returnDirectorySearched.directoryPaths = append(returnDirectorySearched.directoryPaths, path.Join(dirname, foundPath.Name()))
					} else {
						if fileBytes, err := ioutil.ReadFile(path.Join(dirname, foundPath.Name())); err == nil {
							returnDirectorySearched.fileHashes = append(returnDirectorySearched.fileHashes, sha1.Sum(fileBytes))
						}
					}
				}
				pathChan <- returnDirectorySearched
			}
		}
	}

	for len(checkPaths) != 0 || currentChanNumber != 0 {
		if len(checkPaths) != 0 && currentChanNumber != maxChanNumber {
			checkPath := checkPaths[0]
			checkPaths = append(checkPaths[:0], checkPaths[1:]...)

			go getFileHashes(checkPath)
			currentChanNumber++
		} else {
			pathSearch := <-pathChan
			currentChanNumber--

			checkPaths = append(checkPaths, pathSearch.directoryPaths...)
			for _, fileHash := range pathSearch.fileHashes {
				hasher.Write(fileHash[:])
			}
		}
	}

	return fmt.Sprintf("%x", hasher.Sum(nil))
}

func fileSearcher(findFile, rootPath string, maxChanNumber int) []string {
	type directorySearch struct {
		directoryPaths []string
		searchPaths    []string
	}

	if maxChanNumber < 1 {
		maxChanNumber = runtime.GOMAXPROCS(0)
	}

	currentChanNumber := 0
	pathChan := make(chan directorySearch, maxChanNumber)
	checkPaths := []string{rootPath}
	foundPaths := make([]string, 0)

	checkDirectoryForSearchFile := func(searchFile, dirname string) {
		if f, err := os.Open(dirname); err != nil {
			pathChan <- directorySearch{}
		} else {
			foundPaths, err := f.Readdir(-1)
			f.Close()
			if err != nil {
				pathChan <- directorySearch{}
			} else {
				returnDirectorySearched := directorySearch{make([]string, 0), make([]string, 0)}
				for _, foundPath := range foundPaths {
					if foundPath.IsDir() {
						returnDirectorySearched.directoryPaths = append(returnDirectorySearched.directoryPaths, path.Join(dirname, foundPath.Name()))
					} else if foundPath.Name() == searchFile {
						returnDirectorySearched.searchPaths = append(returnDirectorySearched.searchPaths, path.Join(dirname, foundPath.Name()))
					}
				}
				pathChan <- returnDirectorySearched
			}
		}
	}

	for len(checkPaths) != 0 || currentChanNumber != 0 {
		if len(checkPaths) != 0 && currentChanNumber != maxChanNumber {
			checkPath := checkPaths[0]
			checkPaths = append(checkPaths[:0], checkPaths[1:]...)

			go checkDirectoryForSearchFile(findFile, checkPath)
			currentChanNumber++
		} else {
			pathSearch := <-pathChan
			currentChanNumber--

			checkPaths = append(checkPaths, pathSearch.directoryPaths...)
			foundPaths = append(foundPaths, pathSearch.searchPaths...)
		}
	}
	return foundPaths
}

func generateID() string {
	var buffer bytes.Buffer
	rnd := rand.New(rand.NewSource(time.Now().UnixNano()))

	buffer.WriteRune('r')
	for index := 0; index < 9; index++ {
		buffer.WriteString(fmt.Sprintf("%d", rnd.Int()%10))
	}
	buffer.WriteRune('j')
	return buffer.String()
}

func getDirMap(rootDir, dirName string, fromRoot uint64) dirMap {
	directory, err := os.Open(path.Join(rootDir, dirName))

	if err != nil {
		return dirMap{Dir: path.Join(rootDir, dirName), DirName: dirName, Dirs: make([]dirMap, 0), FromRoot: fromRoot, Weight: fromRoot}
	}

	foundPaths, err := directory.Readdir(-1)
	directory.Close()

	if err != nil {
		return dirMap{Dir: path.Join(rootDir, dirName), DirName: dirName, Dirs: make([]dirMap, 0), FromRoot: fromRoot, Weight: fromRoot}
	}

	returnDirMap := dirMap{Dir: path.Join(rootDir, dirName), DirName: dirName, Dirs: make([]dirMap, 0), FromRoot: fromRoot, Weight: fromRoot}

	for _, foundPath := range foundPaths {
		if foundPath.IsDir() {
			subDirMap := getDirMap(path.Join(rootDir, dirName), foundPath.Name(), fromRoot+1)
			returnDirMap.Weight += subDirMap.Weight
			returnDirMap.Dirs = append(returnDirMap.Dirs, subDirMap)
		}
	}

	sort.Sort(returnDirMap)

	return returnDirMap
}

func getGithubToken(path string) (string, error) {
	file, err := os.Open(path)
	defer file.Close()

	if err != nil {
		return "", err
	}

	token := githubToken{}

	if err := json.NewDecoder(file).Decode(&token); err != nil {
		return "", err
	}

	return token.Token, nil
}

func getLocalProjectCommit(projectPath string) (string, error) {
	repository, err := git.PlainOpen(projectPath)

	if err != nil {
		return "", err
	}

	ref, err := repository.Head()

	if err != nil {
		return "", err
	}

	remoteHash := ref.Hash()

	return remoteHash.String(), nil
}

func getProjectDescription(projectName, token string) (string, error) {
	requestQuery := query{fmt.Sprintf(descriptionQuery, projectName)}
	buffer := new(bytes.Buffer)
	json.NewEncoder(buffer).Encode(requestQuery)

	request, err := http.NewRequest("POST", "https://api.github.com/graphql", buffer)

	if err != nil {
		return "", err
	}

	request.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))

	client := &http.Client{}

	queryResponse, err := client.Do(request)

	if err != nil {
		return "", err
	}

	descriptionResponse := response{}

	err = json.NewDecoder(queryResponse.Body).Decode(&descriptionResponse)

	if err != nil {
		return "", err
	}

	return descriptionResponse.Data.Repository.Description, nil
}

func getProjectIndex(identifier string, projects []RJProject) int {
	for index, project := range projects {
		if project.URL == identifier || strings.ToLower(project.Name) == strings.ToLower(path.Base(identifier)) || project.ID == identifier {
			return index
		}
	}
	return -1
}

func getRemoteProjectCommit(projectURL string) (string, error) {
	repository, err := git.Clone(memory.NewStorage(), nil, &git.CloneOptions{
		URL: projectURL,
	})

	if err != nil {
		return "", err
	}

	ref, err := repository.Head()

	if err != nil {
		return "", err
	}

	remoteHash := ref.Hash()

	return remoteHash.String(), nil
}

func getRjGlobal(projectRootPath string) (RJGlobal, error) {
	var rjGlobal RJGlobal

	if _, err := os.Stat(path.Join(projectRootPath, "RJglobal.json")); err != nil {
		return RJGlobal{}, newErrRjFileNotFound(true, false)
	}

	rjGlobalFile, err := os.Open(path.Join(projectRootPath, "RJglobal.json"))

	defer rjGlobalFile.Close()

	if err != nil {
		return RJGlobal{}, err
	}

	if err := json.NewDecoder(rjGlobalFile).Decode(&rjGlobal); err != nil {
		return RJGlobal{}, err
	}

	return rjGlobal, nil
}

func getRjInfo(projectRootPath string) (*RJInfo, error) {
	var err error
	var rjInfo RJInfo

	if rjInfo.RJGlobal, err = getRjGlobal(projectRootPath); err != nil {
		return &rjInfo, err
	}

	if rjInfo.RJLocal, err = getRjLocal(projectRootPath); err != nil {
		if _, ok := err.(*errRjFileNotFound); ok {
			rjInfo.RJLocal, err = initializeLocal(projectRootPath, rjInfo.RJGlobal, false)
		} else {
			return &rjInfo, err
		}
	}

	return &rjInfo, err
}

func getRjLocal(projectRootPath string) (RJLocal, error) {
	if _, err := os.Stat(path.Join(projectRootPath, "RJglobal.json")); err != nil {
		return RJLocal{}, newErrRjFileNotFound(false, true)
	}

	rjLocalFile, err := os.Open(path.Join(projectRootPath, "RJlocal.json"))

	defer rjLocalFile.Close()

	if err != nil {
		return RJLocal{}, err
	}

	rjLocal := RJLocal{Projects: make(map[string]RJLocalProject)}

	if err := json.NewDecoder(rjLocalFile).Decode(&rjLocal); err != nil {
		return RJLocal{}, err
	}

	return rjLocal, nil
}

func handleCloneProject(rjProject *RJProject, rjLocal *RJLocal, force bool) error {
	rjLocalProject, rjLocalProjectExists := rjLocal.Projects[rjProject.ID]

	if !rjLocalProjectExists {
		return fmt.Errorf("project '%s' does not exist locally", rjProject.Name)
	}

	if rjLocalProject.Path == "" {
		return fmt.Errorf("project '%s' needs a local path before it can be cloned", rjProject.Name)
	}

	_, err := getLocalProjectCommit(rjLocalProject.Path)

	if err == nil {
		if force {
			if err := cloneProject(rjLocalProject.Path, rjProject.URL); err == nil {
				fmt.Printf("Project '%s' has been successfully cloned to %s.", rjProject.Name, rjLocalProject.Path)
			} else {
				return errors.Wrapf(err, "problem cloning Project '%s'", rjProject.Name)
			}
		} else {
			return fmt.Errorf("could not clone project '%s' because it already exists, please specify '-force' if you wish to overwrite", rjProject.Name)
		}
	} else {
		if err == git.ErrRepositoryNotExists {
			if err := cloneProject(rjLocalProject.Path, rjProject.URL); err == nil {
				fmt.Printf("Project '%s' has been successfully cloned to %s.", rjProject.Name, rjLocalProject.Path)
			} else {
				return errors.Wrapf(err, "problem cloning Project '%s'", rjProject.Name)
			}
		} else {
			return errors.Wrapf(err, "problem cloning Project '%s'", rjProject.Name)
		}
	}

	return nil
}

func handleSyncronizeLocal(rjProject *RJProject, rjLocal *RJLocal) (string, error) {
	rjLocalProject, rjLocalProjectExists := rjLocal.Projects[rjProject.ID]

	if !rjLocalProjectExists || rjLocalProject.Path == "" {
		return "", fmt.Errorf("project '%s' does not exist locally", rjProject.Name)
	}

	newlySynced, err := syncronizeLocal(*rjProject, rjLocalProject)

	if err != nil {
		return "", errors.Wrapf(err, "problem syncing Project '%s'", rjProject.Name)
	}

	if newlySynced {
		return fmt.Sprintf("Project '%s' has been synced.", rjProject.Name), nil
	}

	return fmt.Sprintf("Project '%s' is already in sync.", rjProject.Name), nil
}

func initializeGlobal(projectRootPath string, force bool) (RJGlobal, error) {
	if _, err := os.Stat(projectRootPath); err != nil {
		return RJGlobal{}, errors.New("path to project root does not exist")
	}

	if _, err := os.Stat(path.Join(projectRootPath, "RJglobal.json")); err == nil && !force {
		return RJGlobal{}, errors.New("path to RJglobal file already exists, add the -force flag to overwrite the current config file")
	}

	rjGlobalFile, err := os.Create(path.Join(projectRootPath, "RJglobal.json"))

	defer rjGlobalFile.Close()

	if err != nil {
		return RJGlobal{}, err
	}

	rjGlobal := RJGlobal{Projects: make([]RJProject, 0)}

	if err = json.NewEncoder(rjGlobalFile).Encode(&rjGlobal); err != nil {
		return RJGlobal{}, errors.Wrap(err, "error initializing RJglobal file")
	}

	return rjGlobal, nil
}

func initializeLocal(projectRoot string, rjGlobal RJGlobal, force bool) (RJLocal, error) {
	if _, err := os.Stat(projectRoot); err != nil {
		return RJLocal{}, errors.New("Path to project root does not exist")
	}

	if _, err := os.Stat(path.Join(projectRoot, "RJlocal.json")); err == nil && !force {
		return RJLocal{}, errors.New("path to RJlocal file already exists, add the -force flag to overwrite the current local file")
	}

	rjLocalFile, err := os.Create(path.Join(projectRoot, "RJlocal.json"))

	defer rjLocalFile.Close()

	if err != nil {
		return RJLocal{}, err
	}

	rjLocal := RJLocal{Projects: make(map[string]RJLocalProject), SearchPaths: make([]string, 0)}

	for _, rjProject := range rjGlobal.Projects {
		rjLocal.Projects[rjProject.ID] = RJLocalProject{}
	}

	if err = json.NewEncoder(rjLocalFile).Encode(&rjLocal); err != nil {
		return RJLocal{}, errors.Wrap(err, "error initializing RJglobal file")
	}

	return rjLocal, nil
}

func killClones() error {
	robLocation := os.Args[0]

	var command *exec.Cmd
	if runtime.GOOS == "windows" {
		command = exec.Command("TASKKILL", "/F", "/T", "/IM", filepath.Base(robLocation), "/FI", fmt.Sprintf("PID ne %d", os.Getpid()))
	} else {
		command = exec.Command("bash", "-c", fmt.Sprintf(`ps -aux | grep %s | awk '{if ($2 != %d) print $2}' | sudo xargs kill -9`, filepath.Base(robLocation), os.Getpid()))
	}

	command.Stdout = os.Stdout

	return command.Run()
}

func killServer() error {
	var command *exec.Cmd
	if runtime.GOOS == "windows" {
		command = exec.Command("TASKKILL", "/F", "/T", "/IM", fmt.Sprintf("%s.exe", rjServer))
	} else {
		command = exec.Command("bash", "-c", fmt.Sprintf(`ps -aux | grep %s | awk '{print $2}' | sudo xargs kill -9`, rjServer))
	}

	command.Stdout = os.Stdout

	err := command.Run()

	return err
}

func localProjectSynced(localProjectPath, projectURL, projectName string) (bool, error) {
	fileInfo, err := os.Lstat(localProjectPath)

	if err != nil {
		return false, errors.Wrapf(err, "problem with the project path for project '%s'.\n", projectName)
	}

	if !fileInfo.IsDir() {
		return false, errors.New(fmt.Sprintf("Project path for project '%s' is not a directory.\n", projectName))
	}

	localProjectHash, err := getLocalProjectCommit(localProjectPath)

	if err != nil {
		return false, errors.Wrapf(err, "could not get commit hash of local project '%s'.\n", projectName)

	}

	remoteProjectHash, err := getRemoteProjectCommit(projectURL)

	if err != nil {
		return false, errors.Wrapf(err, "Could not get remote commit hash of project '%s'.\n", projectName)
	}

	return localProjectHash == remoteProjectHash, nil
}

// Given that the 'killChannel' channel has already been created and registered with signal.Notify,
// this will handle killing the created process in the case this program is ended via CTRL+C
func manageProcessReaping(command *exec.Cmd, killChannel chan os.Signal) {
	signal := <-killChannel
	if signal.String() == "RJ" {
		return
	}

	if command.Process != nil {
		if runtime.GOOS == "windows" {
			subcommand := exec.Command("TASKKILL", "/T", "/F", "/PID", strconv.Itoa(command.Process.Pid))
			subcommand.Stdout = os.Stdout
			subcommand.Stderr = os.Stderr
			err := subcommand.Run()
			if err != nil {
				fmt.Println(err)
			}
		} else {
			command.Process.Kill()
		}
	}
}

// Given that the 'killChannel' channel has already been created and registered with signal.Notify,
// this will handle killing a docker run command with a name matching the provided name param in
// the case this program is ended via CTRL+C
func manageRunReaping(name string, killChannel chan os.Signal) {
	signal := <-killChannel
	if signal.String() == "RJ" {
		return
	}

	if err := exec.Command("docker", "stop", name).Run(); err != nil {
		fmt.Println(err)
	}
}

func newDirMap(rootDir string) dirMap {
	return getDirMap(filepath.Dir(rootDir), filepath.Base(rootDir), 0)
}

func prettyPrintStruct(structure interface{}, spaces uint64) error {
	bytes, err := json.MarshalIndent(structure, "", strings.Repeat(" ", int(spaces)))

	if err != nil {
		return err
	}

	fmt.Println(string(bytes))
	return nil
}

func printProject(project RJProject, localProjects RJLocal, spaces uint64) error {
	fmt.Printf("\n%s\n%s\n", project.Name, strings.Repeat("=", len(project.Name)))
	err := prettyPrintStruct(project, spaces)

	if err != nil {
		fmt.Println(errors.Wrap(err, "could not print the project: "))
		fmt.Printf("%s\n", strings.Repeat("_", len(project.Name)))
		return err
	}

	if localProject, localProjectExists := localProjects.Projects[project.ID]; localProjectExists {
		fmt.Printf("%s\n", strings.Repeat("-", len(project.Name)))
		err = prettyPrintStruct(localProject, spaces)

		if err != nil {
			fmt.Println(errors.Wrap(err, "could not print the local project"))
			fmt.Printf("%s\n", strings.Repeat("_", len(project.Name)))
			return err
		}
	}

	fmt.Printf("%s\n", strings.Repeat("_", len(project.Name)))
	return nil
}

func pruneLocal(rjInfo *RJInfo) int {
	configProjects := make(map[string]bool)
	pruned := 0

	for _, project := range rjInfo.RJGlobal.Projects {
		configProjects[project.ID] = true
	}

	for projectID := range rjInfo.RJLocal.Projects {
		if !configProjects[projectID] {
			delete(rjInfo.RJLocal.Projects, projectID)
			pruned++
		}
	}

	return pruned
}

func removeContents(directoryPath string) error {
	directory, err := os.Open(directoryPath)

	if err != nil {
		return err
	}

	fileNames, err := directory.Readdirnames(-1)
	directory.Close()

	if err != nil {
		return err
	}

	for _, fileName := range fileNames {
		err = os.RemoveAll(filepath.Join(directoryPath, fileName))
		if err != nil {
			return err
		}
	}

	return nil
}

func removeProjectLocally(project string, rjInfo *RJInfo) error {
	var err error
	index := getProjectIndex(project, rjInfo.RJGlobal.Projects)

	if index == -1 {
		return errors.New("project specified does not exist")
	}

	projectID := rjInfo.RJGlobal.Projects[index].ID
	projectName := rjInfo.RJGlobal.Projects[index].Name
	projectPath := rjInfo.RJLocal.Projects[projectID].Path

	_, localProjectExists := rjInfo.RJLocal.Projects[projectID]

	if !localProjectExists {
		return fmt.Errorf("project %s does not exist locally", projectName)
	}

	if rjInfo.RJLocal.Projects[projectID].Path != "" {
		if _, err = os.Stat(filepath.Join(projectPath, ".RJtag")); err == nil {
			err = os.Remove(path.Join(rjInfo.RJLocal.Projects[projectID].Path, ".RJtag"))

			if err != nil {
				return err
			}
		}
	}

	delete(rjInfo.RJLocal.Projects, projectID)
	fmt.Printf("Deleted Project %s Locally.\n", projectName)

	return nil
}

func rjBuild(rjInfo *RJInfo, rjProject RJProject, projectRoot string, force bool) (bool, error) {
	rjLocalProject, rjLocalProjectExists := rjInfo.RJLocal.Projects[rjProject.ID]

	if rjLocalProjectExists && rjLocalProject.Path != "" {
		if rjLocalProject.LastBuildHash != "" {
			dontBuild := checkProjectBuildHash(rjLocalProject.LastBuildHash, rjLocalProject.Path)

			if dontBuild {
				if !force {
					fmt.Printf("Build hash for Project '%s' is the same as the previous build hash, building skipped; to force building, specify the '-force' flag.\n", rjProject.Name)
					return false, nil
				}
				fmt.Printf("Build hash for Project '%s' is the same as the previous build hash, build is being forced.", rjProject.Name)
			} else {
				fmt.Printf("Build hash for Project '%s' is different from the previous build hash, rebuilding.", rjProject.Name)
			}

			newBuildHash, err := buildProjectLocally(rjLocalProject.Path, projectRoot, rjProject.SitePath)

			if err != nil {
				return false, errors.Wrapf(err, "problem building Project '%s'", rjProject.Name)
			}

			fmt.Printf("Project '%s' successfully built to sitepath '%s'.\n", rjProject.Name, rjProject.SitePath)
			rjLocalProject.LastBuildHash = newBuildHash
			rjInfo.RJLocal.Projects[rjProject.ID] = rjLocalProject
			return true, nil
		}

		fmt.Printf("Project '%s' does not have previous build hash, building now.\n", rjProject.Name)

		newBuildHash, err := buildProjectLocally(rjLocalProject.Path, projectRoot, rjProject.SitePath)

		if err != nil {
			return false, errors.Wrapf(err, "problem building Project '%s'", rjProject.Name)
		}

		fmt.Printf("Project '%s' successfully built to sitepath '%s'.\n", rjProject.Name, rjProject.SitePath)
		rjLocalProject.LastBuildHash = newBuildHash
		rjInfo.RJLocal.Projects[rjProject.ID] = rjLocalProject
		return true, nil
	}

	fmt.Printf("Project '%s' does not exist locally, building in container.\n", rjProject.Name)

	remoteCommit, err := getRemoteProjectCommit(rjProject.URL)

	if err != nil {
		return false, errors.Wrapf(err, "problem getting the remote hash for Project '%s'", rjProject.Name)
	}

	if rjLocalProjectExists && rjLocalProject.LastBuildCommit != "" {
		if remoteCommit != rjLocalProject.LastBuildCommit {
			err = buildProjectRemotely(projectRoot, rjProject.SitePath, rjProject.URL)

			if err != nil {
				return false, errors.Wrapf(err, "problem building Project '%s' remotely", rjProject.Name)
			}

			fmt.Printf("Project '%s' has been successfully cloned to %s.\n", rjProject.Name, rjProject.SitePath)
			rjLocalProject.LastBuildCommit = remoteCommit
			rjInfo.RJLocal.Projects[rjProject.ID] = rjLocalProject
			return true, nil
		}

		if force {
			fmt.Printf("Remote hash for Project '%s' is the same as the previous build's remote commit hash, build is being forced.", rjProject.Name)

			err = buildProjectRemotely(projectRoot, rjProject.SitePath, rjProject.URL)

			if err != nil {
				return false, errors.Wrapf(err, "problem building Project '%s' remotely", rjProject.Name)
			}

			fmt.Printf("Project '%s' has been successfully cloned to %s.\n", rjProject.Name, rjProject.SitePath)
			rjLocalProject.LastBuildCommit = remoteCommit
			rjInfo.RJLocal.Projects[rjProject.ID] = rjLocalProject
			return true, nil
		}
		fmt.Printf("Remote hash for Project '%s' is the same as the previous build's remote commit hash, building skipped; to force building, specify the '-force' flag.\n", rjProject.Name)
		return false, nil

	}

	err = buildProjectRemotely(projectRoot, rjProject.SitePath, rjProject.URL)

	if err != nil {
		return false, errors.Wrapf(err, "problem building Project '%s' remotely", rjProject.Name)
	}

	fmt.Printf("Project '%s' has been successfully cloned to %s.\n", rjProject.Name, rjProject.SitePath)
	rjInfo.RJLocal.Projects[rjProject.ID] = RJLocalProject{LastBuildCommit: remoteCommit}
	return true, nil
}

func rjPushRob(tag string, local bool) error {
	// Equivalent to "build --no-cache -t {tag} -f - /path/to/rob"
	runRobInstallerArgs := []string{
		"build", "--no-cache", "-t", fmt.Sprintf("therileyjohnson/rob:%s", tag),
		"-f", "-", ".",
	}

	cmd := exec.Command("docker", runRobInstallerArgs...)

	if local {
		cmd.Stdin = bytes.NewBufferString(robInstallBuilderLocal)
	} else {
		cmd.Stdin = bytes.NewBufferString(robInstallBuilderRemote)
	}

	cmd.Stdout = os.Stdout

	killChannel := make(chan os.Signal, 1)

	signal.Notify(killChannel,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT,
	)

	err := cmd.Start()

	if err != nil {
		return err
	}

	go manageProcessReaping(cmd, killChannel)

	err = cmd.Wait()

	// Indicates that 'manageProcessReaping' can exit
	killChannel <- RJSignal{}

	if err != nil {
		return err
	}

	//Equivalent to "push therileyjohnson/{tag}"
	runRobPushArgs := []string{
		"push", fmt.Sprintf("therileyjohnson/rob:%s", tag),
	}

	cmd = exec.Command("docker", runRobPushArgs...)

	cmd.Stdout = os.Stdout

	err = cmd.Start()

	if err != nil {
		return err
	}

	go manageProcessReaping(cmd, killChannel)

	err = cmd.Wait()

	// Indicates that 'manageProcessReaping' can exit
	killChannel <- RJSignal{}

	return err
}

func runServer(projectRoot string) (int, error) {
	absRoot, err := filepath.Abs(projectRoot)

	if err != nil {
		return 1, err
	}

	buildName := rjServer

	if runtime.GOOS == "windows" {
		buildName += ".exe"
	}

	cmd := exec.Command(path.Join(absRoot, buildName))

	cmd.Dir = absRoot

	cmd.Stdout = os.Stdout

	err = cmd.Start()

	if err != nil {
		return 1, err
	}

	err = cmd.Wait()

	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			return exitError.Sys().(syscall.WaitStatus).ExitStatus(), err
		}
		return 1, err
	}

	return 0, err
}

func syncronizeLocal(project RJProject, localProject RJLocalProject) (bool, error) {
	localProjectIsSynced, err := localProjectSynced(localProject.Path, project.URL, project.Name)

	if err != nil {
		return false, err
	}

	if localProjectIsSynced {
		return false, nil
	}

	repository, err := git.PlainOpen(localProject.Path)

	if err != nil {
		return false, err
	}

	workingTree, err := repository.Worktree()

	if err != nil {
		return false, err
	}

	err = workingTree.Pull(&git.PullOptions{RemoteName: "origin"})

	if err != nil {
		return false, errors.Wrapf(err, "problem pulling to local repo for project '%s'", project.Name)
	}

	return true, nil
}

func writeUpdate(rootPath string, rjInfo RJInfo) error {
	rjGlobalFile, err := os.Create(path.Join(rootPath, "RJglobal.json"))

	defer rjGlobalFile.Close()

	if err != nil {
		return err
	}

	if err = json.NewEncoder(rjGlobalFile).Encode(&rjInfo.RJGlobal); err != nil {
		return err
	}

	rjLocalFile, err := os.Create(path.Join(rootPath, "RJlocal.json"))

	defer rjLocalFile.Close()

	if err != nil {
		return err
	}

	if err = json.NewEncoder(rjLocalFile).Encode(&rjInfo.RJLocal); err != nil {
		return err
	}

	return nil
}

func writeRjTag(projectID, localPath string) error {
	if _, err := os.Stat(path.Join(localPath, ".RJtag")); err == nil {
		if rjTagFileBytes, err := ioutil.ReadFile(path.Join(localPath, ".RJtag")); err == nil {
			if string(rjTagFileBytes) == projectID {
				return nil
			}
		}
	}

	rjTagFile, err := os.Create(path.Join(localPath, ".RJtag"))

	defer rjTagFile.Close()

	if err != nil {
		return errors.Wrap(err, "Could not automatically generate .RJtag file in the new project's local path.")
	}

	_, err = rjTagFile.WriteString(projectID)

	return err
}
