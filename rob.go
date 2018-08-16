package main

import (
	"bytes"
	"crypto/sha1"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/url"
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

const (
	reactLocalDockerfile  = "react-local-build.dockerfile"
	reactRemoteDockerfile = "react-remote-build.dockerfile"
	rjServer              = "RJserver"
	rjURL                 = "https://therileyjohnson.com"
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
	return d.Dirs[i].Weight > d.Dirs[j].Weight
}

func (d dirMap) Swap(i, j int) {
	d.Dirs[i], d.Dirs[j] = d.Dirs[j], d.Dirs[i]
}

func (d dirMap) String() string {
	return d.StringyRoot(4)
}

func (d dirMap) Stringy(last bool, rootOffset, spaces uint64) string {
	rString := strings.Builder{}
	if last {
		rString.WriteString(strings.Repeat(strings.Repeat(" ", int(spaces)), int((d.FromRoot - 1 - rootOffset))))
		rString.WriteString("|\n")
		rString.WriteString(strings.Repeat(strings.Repeat(" ", int(spaces)), int((d.FromRoot - 1 - rootOffset))))
		rString.WriteString("o" + strings.Repeat("-", int(spaces)-1) + ">")
	} else {
		rString.WriteString(strings.Repeat("|"+strings.Repeat(" ", int(spaces)-1), int((d.FromRoot - rootOffset))))
		rString.WriteString("\n")
		rString.WriteString(strings.Repeat("|"+strings.Repeat(" ", int(spaces)-1), int((d.FromRoot - 1 - rootOffset))))
		rString.WriteString("o" + strings.Repeat("-", int(spaces)-1) + ">")
	}
	rString.WriteString("/" + d.DirName)
	rString.WriteString("\n")

	for _, sd := range d.Dirs {
		rString.WriteString(sd.Stringy(last, rootOffset, spaces))
	}

	return rString.String()
}

func (d dirMap) StringyRoot(spaces uint64) string {
	rString := strings.Builder{}
	rString.WriteString("+")
	rString.WriteString(d.Dir)
	rString.WriteString("\n")

	for index, sd := range d.Dirs {
		rString.WriteString(sd.Stringy(index == len(d.Dirs)-1, d.FromRoot, spaces))
	}

	return rString.String()
}

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

// Generated by https://quicktype.io

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

// func buildProjectLocally(localPath, rootPath, sitePath string) (string, error) {
// 	return buildProject(localPath, rootPath, sitePath, "", false)
// }

// func buildProjectRemotely(rootPath, sitePath, githubURL string) error {
// 	_, err := buildProject("", rootPath, sitePath, githubURL, true)
// 	return err
// }

// func buildProject(localPath, rootPath, sitePath, githubURL string, remote bool) (string, error) {
// 	newHash := ""

// 	if !remote {
// 		newHash = dasher(localPath, -1)
// 	}

// 	absRoot, err := filepath.Abs(rootPath)

// 	if err != nil {
// 		return "", err
// 	}

// 	var imageBuildArgs []string
// 	if remote {
// 		// Equivalent to:
// 		// docker build -t rjtest:latest \
// 		// --build-arg GITHUB_DIR={Project Name} --build-arg GITHUB_URL={Project Github URL} \
// 		// -f - {path to build context}
// 		imageBuildArgs = []string{
// 			"build", "-t", "rj-react-build:latest",
// 			"--build-arg", fmt.Sprintf("GITHUB_DIR=%s", path.Base(githubURL)),
// 			"--build-arg", fmt.Sprintf("GITHUB_URL=%s", githubURL),
// 			"-f", "-", filepath.Clean(localPath),
// 		}
// 	} else {
// 		// Equivalent to "docker build -t rj-react-build:latest -f - {path to build context}"
// 		imageBuildArgs = []string{
// 			"build", "-t", "rj-react-build:latest",
// 			"-f", "-",
// 			filepath.Clean(localPath),
// 		}
// 	}

// 	cmd := exec.Command("docker", imageBuildArgs...)

// 	if remote {
// 		cmd.Stdin = bytes.NewBufferString(remoteReactBuild)
// 	} else {
// 		cmd.Stdin = bytes.NewBufferString(localReactBuild)
// 	}

// 	cmd.Stdout = os.Stdout

// 	killChannel := make(chan os.Signal, 1)

// 	signal.Notify(killChannel,
// 		syscall.SIGHUP,
// 		syscall.SIGINT,
// 		syscall.SIGTERM,
// 		syscall.SIGQUIT,
// 	)

// 	err = cmd.Start()

// 	if err != nil {
// 		return "", err
// 	}

// 	go manageProcessReaping(cmd, killChannel)

// 	err = cmd.Wait()

// 	// Indicates that 'manageProcessReaping' can exit
// 	killChannel <- RJSignal{}

// 	if err != nil {
// 		return "", err
// 	}

// 	dockerBuildName := generateID()

// 	// Equivalent to "run --rm -v "{local RJsite path}/{project site path}:/app/build" rj-react-build:latest"
// 	runBuildArgs := []string{
// 		"run", "--rm",
// 		"-v", fmt.Sprintf(`%s:/app/build`, filepath.Clean(path.Join(absRoot, sitePath))),
// 		"--name", dockerBuildName,
// 		"rj-react-build:latest",
// 	}

// 	cmd = exec.Command("docker", runBuildArgs...)

// 	cmd.Stdout = os.Stdout

// 	err = cmd.Start()

// 	if err != nil {
// 		return "", err
// 	}

// 	go manageRunReaping(dockerBuildName, killChannel)

// 	err = cmd.Wait()

// 	// Indicates that 'manageProcessReaping' can exit
// 	killChannel <- RJSignal{}

// 	return newHash, err
// }

// func buildRoot(rootPath string) error {
// 	buildName, goArch, goOS := rjServer, runtime.GOARCH, runtime.GOOS

// 	if goOS == "windows" {
// 		buildName += ".exe"
// 	}

// 	// Equivalent to "build -t rj-root-build:latest --build-arg GOOS=windows --build-arg GOARCH=amd64 --build-arg BUILD_NAME=rjtest -f -""
// 	runRootBuildArgs := []string{
// 		"build", "-t", "rj-root-build:latest",
// 		"--build-arg", fmt.Sprintf("BUILD_NAME=%s", buildName),
// 		"--build-arg", fmt.Sprintf("GOARCH=%s", goArch),
// 		"--build-arg", fmt.Sprintf("GOOS=%s", goOS),
// 		"-f", "-", rootPath,
// 	}

// 	cmd := exec.Command("docker", runRootBuildArgs...)

// 	cmd.Stdin = bytes.NewBufferString(rootBuild)

// 	cmd.Stdout = os.Stdout

// 	killChannel := make(chan os.Signal, 1)

// 	signal.Notify(killChannel,
// 		syscall.SIGHUP,
// 		syscall.SIGINT,
// 		syscall.SIGTERM,
// 		syscall.SIGQUIT,
// 	)

// 	err := cmd.Start()

// 	if err != nil {
// 		return err
// 	}

// 	go manageProcessReaping(cmd, killChannel)

// 	err = cmd.Wait()

// 	// Indicates that 'manageProcessReaping' can exit
// 	killChannel <- RJSignal{}

// 	if err != nil {
// 		return err
// 	}

// 	dockerBuildName := generateID()

// 	//Equivalent to "run --rm rj-root-build:latest"
// 	runRootTransferArgs := []string{
// 		"run", "--rm", "--name", dockerBuildName, "rj-root-build:latest",
// 	}

// 	cmd = exec.Command("docker", runRootTransferArgs...)

// 	serverExecutable, err := os.Create(buildName)

// 	if err != nil {
// 		return err
// 	}

// 	cmd.Stdout = serverExecutable

// 	err = cmd.Start()

// 	if err != nil {
// 		return err
// 	}

// 	go manageRunReaping(dockerBuildName, killChannel)

// 	err = cmd.Wait()

// 	// Indicates that 'manageProcessReaping' can exit
// 	killChannel <- RJSignal{}

// 	return err
// }

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

func checkProjectExistance(identifier string, projects []RJProject) bool {
	for _, project := range projects {
		if project.URL == identifier {
			return true
		}
	}
	return false
}

func cloneProject(rjLocalProjectPath, rjProjectURL string) error {
	removeContents(rjLocalProjectPath)
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

func correctPackageJSON(projectLocalPath, projectSitePath string) error {
	packageJSONBytes, err := ioutil.ReadFile(path.Join(projectLocalPath, "package.json"))

	if err != nil {
		return err
	}

	var localProjectNpmPackage npmPackage

	if err = json.Unmarshal(packageJSONBytes, &localProjectNpmPackage); err != nil {
		return err
	}

	localProjectNpmPackage.Homepage = path.Join(rjURL, projectSitePath)

	packageJSONBytes, err = json.Marshal(localProjectNpmPackage)

	if err != nil {
		return err
	}

	err = ioutil.WriteFile(path.Join(projectLocalPath, "package.json"), packageJSONBytes, os.ModePerm)

	return err
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

func getProjectIndex(identifier string, projects []RJProject) int {
	for index, project := range projects {
		if project.URL == identifier || strings.ToLower(project.Name) == strings.ToLower(path.Base(identifier)) || project.ID == identifier {
			return index
		}
	}
	return -1
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

func getRjGlobal(projectRootPath string) (RJGlobal, error) {
	var rjGlobal RJGlobal

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

func getRjLocal(projectRootPath string) (RJLocal, error) {
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
		printAndExit(err)
	}

	rjLocal := RJLocal{Projects: make(map[string]RJLocalProject), SearchPaths: make([]string, 0)}

	for _, rjProject := range rjGlobal.Projects {
		rjLocal.Projects[rjProject.ID] = RJLocalProject{}
	}

	if err = json.NewEncoder(rjLocalFile).Encode(&rjLocal); err != nil {
		printAndExitf("Error initializing RJglobal file: %s", err)
	}

	return rjLocal, nil
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

func newDirMap(rootDir string) dirMap {
	return getDirMap(filepath.Dir(rootDir), filepath.Base(rootDir), 0)
}

func parseArguments() arguments {
	var args arguments
	flag.BoolVar(&args.add, "add", false, "Adds the project specified by 'project' with information provided via command line args 'description', site-path', and 'local-path'.")
	flag.BoolVar(&args.build, "build", false, "Builds either the local project specified by 'project' or all local projects if 'project' is not specified; will check the last build hash prior to building and if they are the same then the project will only be rebuild if the '-force' flag is included.")
	flag.BoolVar(&args.clone, "clone", false, "Clones the local project specified by 'project' if it does not have a local hash or all local projects which do not have a local hash if 'project' is not specified; NOTE: this will clear the whole directory prior to cloning.")
	flag.BoolVar(&args.updateDescription, "desc", false, "Used in conjuction with 'up', update the descriptions for the projects.")
	flag.BoolVar(&args.discover, "discover", false, "Search for RJtag's correlating to the ID's of the projects in RJglobal, starting in 'SearchPath' specified in the RJlocal file.")
	flag.BoolVar(&args.flightCheck, "flight", false, "Prints the output of the 'pre-flight check'.")
	flag.BoolVar(&args.force, "force", false, "Force the operation(s); be careful.")
	flag.BoolVar(&args.initialize, "init", false, "Initialize a new RJglobal file if not already created. Can be forced.")
	flag.BoolVar(&args.initializeLocal, "init-local", false, "Initialize a new RJglobal file if not already created. Can be forced.")
	flag.BoolVar(&args.kill, "kill", false, "Attempts to kill any running server instances.")
	flag.BoolVar(&args.local, "l", false, "Modifies the behaviour of some commands to only have effects on the RJlocal config.")
	flag.BoolVar(&args.list, "ls", false, "Lists the details of the project specified by 'project', or all projects if none are specified or found, as pretty printed JSON.")
	flag.BoolVar(&args.prune, "prune", false, "Deletes all local projects not found in RJglobal.")
	flag.BoolVar(&args.root, "root", false, "Modifies the behavior of some commands to effect the root project.")
	flag.BoolVar(&args.run, "run", false, "Runs the webserver in the project root in management mode (Checks program return code, and checks for update on status code 9).")
	flag.BoolVar(&args.syncronizeLocal, "sync", false, "Checks the local git hash against what's in the remote repo and updates either the local project specified by 'project' or all local projects if 'project' is not specified.")
	flag.BoolVar(&args.suicide, "suicide", false, "Kills all running instances of self, except that which was invoked with '-suicide'.")
	flag.BoolVar(&args.remove, "rm", false, "Removes the project specified by 'project', adding the 'local' flag will only delete it locally..")
	flag.BoolVar(&args.update, "up", false, "Updates the project specified by 'project' with information provided via command line args 'description', site-path', and 'local-path'.")
	flag.BoolVar(&args.upgrade, "upgrade", false, "Attempts to upgrade rob to the newest version.")

	flag.Uint64Var(&args.spaces, "spaces", 4, "The number of spaces desired when printing something out.")

	flag.StringVar(&args.mapDirectory, "map", "", "Path to map, specify indentation desired with '-spaces'.")
	flag.StringVar(&args.updateLocalPath, "lp", "", "Path on the machine, where the project should be built from. No-op if not used in conjunction with 'up' command line argument.")
	flag.StringVar(&args.selectProject, "p", "", "Project selection for creating, reading, updating, and deleting; if creating this should be the Github URL of project you want to add.")
	flag.StringVar(&args.pushat, "pushat", "", "Builds the rob installer dockerfile and pushes it with the tag specified by '-pushat'.")
	flag.StringVar(&args.projectRoot, "rp", "./", "Path to the project root.")
	flag.StringVar(&args.updateSearchPath, "sep", "", "The path where the auto discovery for directories with RJtag's should start searching. No-op if not used in conjunction with 'local' command line argument, as well as 'add', 'up', or 'rm'.")
	flag.StringVar(&args.updateSitePath, "sip", "", "Path relative to the root of the rj website project, where the project should be output to when built. No-op if not used in conjunction with 'up' command line argument.")
	flag.StringVar(&args.tokenFilePath, "t", "./token.json", "Name of the json file in the project root with the gitlab token for gathering the project descriptions, or the token directly.")

	flag.Parse()

	args.tokenFilePath = path.Join(args.projectRoot, args.tokenFilePath)

	return args
}

func parseUpdateSearchPath(inputString string, oldPaths []string) ([]string, error) {
	mapPaths := make(map[int]string, 0)

	if err := json.Unmarshal([]byte(inputString), &mapPaths); err == nil {
		for index, newPath := range mapPaths {
			if index < len(oldPaths) {
				oldPaths[index] = newPath
			}
		}
		return oldPaths, nil
	}

	if err := json.Unmarshal([]byte(inputString), &mapPaths); err == nil {
		for index, newPath := range mapPaths {
			if index < len(oldPaths) {
				oldPaths[index] = newPath
			}
		}
		return oldPaths, nil
	}

	paths := strings.Split(inputString, "|")
	for _, newPath := range paths {
		if strings.Contains(newPath, ":") {
			index, err := strconv.ParseUint(newPath[:strings.Index(newPath, ":")], 10, 64)
			if err != nil {
				continue
			}
			oldPaths[index] = newPath[strings.Index(newPath, ":")+1:]
		}
	}

	return oldPaths, nil
}

func preFlightCheck(projectRootPath string) (bool, bool, error) {
	var rjGlobalExists, rjLocalExists bool

	if _, err := os.Stat(projectRootPath); err != nil {
		return false, false, errors.New("Path to project root does not exist")
	}

	if _, err := os.Stat(path.Join(projectRootPath, "RJglobal.json")); err == nil {
		rjGlobalExists = true
	}

	if _, err := os.Stat(path.Join(projectRootPath, "RJlocal.json")); err == nil {
		rjLocalExists = true
	}

	return rjGlobalExists, rjLocalExists, nil
}

func prettyPrintStruct(structure interface{}) error {
	bytes, err := json.MarshalIndent(structure, "", "  ")

	if err != nil {
		return err
	}

	fmt.Println(string(bytes))
	return nil
}

func prettyPrintLocalProject(localProject RJLocalProject) {
	err := prettyPrintStruct(localProject)
	if err != nil {
		fmt.Println(errors.Wrap(err, "could not print the project").Error())
	}
}

func prettyPrintProject(project RJProject) {
	err := prettyPrintStruct(project)
	if err != nil {
		fmt.Println(errors.Wrap(err, "could not print the project").Error())
	}
}

func printAndExit(print ...interface{}) {
	fmt.Println(print...)
	os.Exit(1)
}

func printAndExitf(formatString string, print ...interface{}) {
	fmt.Printf(formatString, print...)
	os.Exit(1)
}

func printProject(project RJProject, localProjects RJLocal) {
	fmt.Printf("%s\n%s\n", project.Name, strings.Repeat("=", len(project.Name)))
	prettyPrintProject(project)

	if localProject, localProjectExists := localProjects.Projects[project.ID]; localProjectExists {
		fmt.Printf("%s\n", strings.Repeat("-", len(project.Name)))
		prettyPrintLocalProject(localProject)
	}

	fmt.Printf("%s\n", strings.Repeat("=", len(project.Name)))
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

func writeUpdate(projectRootPath string, rjInfo RJInfo) error {
	rjGlobalFile, err := os.Create(path.Join(projectRootPath, "RJglobal.json"))

	defer rjGlobalFile.Close()

	if err != nil {
		return err
	}

	if err = json.NewEncoder(rjGlobalFile).Encode(&rjInfo.RJGlobal); err != nil {
		return err
	}

	rjLocalFile, err := os.Create(path.Join(projectRootPath, "RJlocal.json"))

	defer rjLocalFile.Close()

	if err != nil {
		return err
	}

	if err = json.NewEncoder(rjLocalFile).Encode(&rjInfo.RJLocal); err != nil {
		return err
	}

	return nil
}

func rjBuild(rjInfo *RJInfo, rjProject *RJProject, args *arguments) bool {
	if rjLocalProject, rjLocalProjectExists := rjInfo.RJLocal.Projects[rjProject.ID]; rjLocalProjectExists && rjLocalProject.Path != "" {
		if rjLocalProject.LastBuildHash != "" {
			if dontBuild := checkProjectBuildHash(rjLocalProject.LastBuildHash, rjLocalProject.Path); !dontBuild || args.force {
				if dontBuild {
					fmt.Printf("Build hash for Project '%s' is the same as the previous build hash, build is being forced.", rjProject.Name)
				} else {
					fmt.Printf("Build hash for Project '%s' is different from the previous build hash, rebuilding.", rjProject.Name)
				}

				// newBuildHash, err := buildProjectLocally(rjLocalProject.Path, args.projectRoot, rjProject.SitePath)

				// if err == nil {
				// 	fmt.Printf("Project '%s' successfully built to sitepath '%s'.\n", rjProject.Name, rjProject.SitePath)
				// 	rjLocalProject.LastBuildHash = newBuildHash
				// 	rjInfo.RJLocal.Projects[rjProject.ID] = rjLocalProject
				// 	return true
				// }

				// fmt.Println(errors.Wrapf(err, "problem building Project '%s'", rjProject.Name))
			} else {
				fmt.Printf("Build hash for Project '%s' is the same as the previous build hash, building skipped; to force building, specify the '-force' flag.\n", rjProject.Name)
			}
		} else {
			fmt.Printf("Project '%s' does not have previous build hash, building now.\n", rjProject.Name)

			// newBuildHash, err := buildProjectLocally(rjLocalProject.Path, args.projectRoot, rjProject.SitePath)

			// if err == nil {
			// 	fmt.Printf("Project '%s' successfully built to sitepath '%s'.\n", rjProject.Name, rjProject.SitePath)
			// 	rjLocalProject.LastBuildHash = newBuildHash
			// 	rjInfo.RJLocal.Projects[rjProject.ID] = rjLocalProject
			// 	return true
			// }

			// fmt.Println(errors.Wrapf(err, "problem building Project '%s'", rjProject.Name))
		}
	} else {
		fmt.Printf("Project '%s' does not exist locally, building in container.\n", rjProject.Name)

		if remoteCommit, err := getRemoteProjectCommit(rjProject.URL); err == nil {
			if rjLocalProjectExists && rjLocalProject.LastBuildCommit != "" {
				if remoteCommit != rjLocalProject.LastBuildCommit {
					// if err = buildProjectRemotely(args.projectRoot, rjProject.SitePath, rjProject.URL); err == nil {
					// 	fmt.Printf("Project '%s' has been successfully cloned to %s.\n", rjProject.Name, rjProject.SitePath)
					// 	rjLocalProject.LastBuildCommit = remoteCommit
					// 	rjInfo.RJLocal.Projects[rjProject.ID] = rjLocalProject
					// 	return true
					// }
					fmt.Println(errors.Wrapf(err, "problem building Project '%s' remotely", rjProject.Name))
				} else {
					if args.force {
						fmt.Printf("Remote hash for Project '%s' is the same as the previous build's remote commit hash, build is being forced.", rjProject.Name)

						// err = buildProjectRemotely(args.projectRoot, rjProject.SitePath, rjProject.URL)

						if err == nil {
							fmt.Printf("Project '%s' has been successfully cloned to %s.\n", rjProject.Name, rjProject.SitePath)
							rjLocalProject.LastBuildCommit = remoteCommit
							rjInfo.RJLocal.Projects[rjProject.ID] = rjLocalProject
							return true
						}

						fmt.Println(errors.Wrapf(err, "problem building Project '%s' remotely", rjProject.Name))
					} else {
						fmt.Printf("Remote hash for Project '%s' is the same as the previous build's remote commit hash, building skipped; to force building, specify the '-force' flag.\n", rjProject.Name)
					}
				}
			} else {
				// if err = buildProjectRemotely(args.projectRoot, rjProject.SitePath, rjProject.URL); err == nil {
				// 	fmt.Printf("Project '%s' has been successfully cloned to %s.\n", rjProject.Name, rjProject.SitePath)
				// 	rjInfo.RJLocal.Projects[rjProject.ID] = RJLocalProject{LastBuildCommit: remoteCommit}
				// 	return true
				// }
				fmt.Println(errors.Wrapf(err, "problem building Project '%s' remotely", rjProject.Name))
			}
		} else {
			fmt.Println(errors.Wrapf(err, "problem getting the remote hash for Project '%s'", rjProject.Name))
		}
	}
	return false
}

// func handleBuildRoot(rjInfo *RJInfo, args *arguments) bool {
// 	if _, err := os.Stat(args.projectRoot); err == nil {
// 		if localHash, err := getLocalProjectCommit(args.projectRoot); err == nil {
// 			if localHash != rjInfo.RJLocal.LastRemoteHashOnBuild || args.force {
// 				if remoteHash, err := getRemoteProjectCommit(rjInfo.RJGlobal.URL); err == nil && remoteHash != localHash {
// 					fmt.Println("Local project is not synced with remote, make sure to push/pull as needed.")
// 				}

// 				if err = buildRoot(args.projectRoot); err == nil {
// 					rjInfo.RJLocal.LastRemoteHashOnBuild = localHash
// 					return true
// 				}

// 				fmt.Println(err)
// 			} else {
// 				fmt.Println("Skipping building root project because the last build hash matches the local commit hash, please specify '-force' if you wish to override.")
// 			}
// 		} else {
// 			fmt.Println(errors.Wrap(err, "problem fetching remote commit hash for root project"))
// 		}
// 	} else {
// 		fmt.Println(errors.Wrap(err, "path to project root does not exist"))
// 	}
// 	return false
// }

func handleCloneProject(rjProject *RJProject, rjLocal *RJLocal, force bool) {
	if rjLocalProject, rjLocalProjectExists := rjLocal.Projects[rjProject.ID]; rjLocalProjectExists {
		if rjLocalProject.Path != "" {
			if _, err := getLocalProjectCommit(rjLocalProject.Path); err == nil {
				if force {
					if err := cloneProject(rjLocalProject.Path, rjProject.URL); err == nil {
						fmt.Printf("Project '%s' has been successfully cloned to %s.", rjProject.Name, rjLocalProject.Path)
					} else {
						fmt.Println(errors.Wrapf(err, "problem cloning Project '%s'", rjProject.Name))
					}
				} else {
					fmt.Printf("Could not clone project '%s' because it already exists, please specify '-force' if you wish to overwrite.\n", rjProject.Name)
				}
			} else {
				if err == git.ErrRepositoryNotExists {
					if err := cloneProject(rjLocalProject.Path, rjProject.URL); err == nil {
						fmt.Printf("Project '%s' has been successfully cloned to %s.", rjProject.Name, rjLocalProject.Path)
					} else {
						fmt.Println(errors.Wrapf(err, "problem cloning Project '%s'", rjProject.Name))
					}
				} else {
					fmt.Println(errors.Wrapf(err, "problem cloning Project '%s'", rjProject.Name))
				}
			}
		} else {
			fmt.Printf("Project '%s' needs a local path before it can be cloned.\n", rjProject.Name)
		}
	} else {
		fmt.Printf("Project '%s' does not exist locally.", rjProject.Name)
	}
}

func handleProjectRun(rjInfo *RJInfo, args *arguments) {
	var err error
	var statusCode int

	if !args.kill {
		fmt.Println("Make sure you only have one server instance running, assure this with '-kill'.")
	}

	for statusCode == 0 || statusCode == 9 {
		statusCode, err = runServer(args.projectRoot)
		fmt.Println(statusCode, err)
	}
}

func handleServerReaping() {
	rjReapServer()
}

func handleSyncronizeLocal(rjProject *RJProject, rjLocal *RJLocal) {
	if rjLocalProject, rjLocalProjectExists := rjLocal.Projects[rjProject.ID]; rjLocalProjectExists && rjLocalProject.Path != "" {
		if rjLocalProject.Path != "" {
			if newlySynced, err := syncronizeLocal(*rjProject, rjLocalProject); err == nil {
				if err != nil {
					fmt.Println(errors.Wrapf(err, "Could not syncronize project '%s'", rjProject.Name))
				} else {
					if newlySynced {
						fmt.Printf("Project '%s' has been synced.", rjProject.Name)
					} else {
						fmt.Printf("Project '%s' is already in sync.", rjProject.Name)
					}
				}
			} else {
				fmt.Println(errors.Wrapf(err, "problem syncing Project '%s'", rjProject.Name))
			}
		} else {
			fmt.Printf("Project '%s' needs a local path before it can be synced.\n", rjProject.Name)
		}
	} else {
		fmt.Printf("Project '%s' does not exist locally.", rjProject.Name)
	}
}

func rjDiscover(rjInfo *RJInfo, args *arguments) bool {
	if len(rjInfo.RJLocal.SearchPaths) != 0 {
		discoveredTagPaths := make([]string, 0)

		for _, searchPath := range rjInfo.RJLocal.SearchPaths {
			discoveredTagPaths = append(discoveredTagPaths, fileSearcher(".RJtag", searchPath, -1)...)
		}

		fmt.Printf("Found %d Projects\n", len(discoveredTagPaths))

		errs, forced, found := 0, 0, 0

		for _, discoveredTagPath := range discoveredTagPaths {
			if id, err := ioutil.ReadFile(discoveredTagPath); err != nil {
				fmt.Println(errors.Wrapf(err, "could not get id from tag at %s", discoveredTagPath))
				errs++
			} else {
				_, localProjectExists := rjInfo.RJLocal.Projects[string(id)]
				if !localProjectExists {
					rjInfo.RJLocal.Projects[string(id)] = RJLocalProject{Path: filepath.Dir(discoveredTagPath)}
					found++
				} else if args.force {
					rjInfo.RJLocal.Projects[string(id)] = RJLocalProject{Path: filepath.Dir(discoveredTagPath)}
					forced++
				}
			}
		}

		pruned := pruneLocal(rjInfo)

		desc := "kept"
		result := found - pruned

		if args.force {
			desc = "forced"
			result = forced - pruned
		}

		if result < 0 {
			result = 0
		}

		fmt.Printf("%d Projects found and %s, %d pruned, and %d had errors occur reading the tags.\n", result, desc, pruned, errs)

		return true
	}
	fmt.Println("Cannot discover if the list of paths to search in is empty (RJlocal.SearchPaths).")
	return false
}

func rjPrune(rjInfo *RJInfo) bool {
	pruned := pruneLocal(rjInfo)

	if pruned == 0 {
		fmt.Println("No Local Projects Pruned.")
	} else {
		fmt.Printf("%d Local Projects Pruned.\n", pruned)
		return true
	}
	return false
}

func rjKillClones() error {
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

// func rjPushRob(tag string) error {
// 	robLocation := filepath.Dir(os.Args[0])

// 	// Equivalent to "build --no-cache -t {tag} -f - /path/to/rob"
// 	runRobInstallerArgs := []string{
// 		"build", "--no-cache", "-t", fmt.Sprintf("therileyjohnson/rob:%s", tag),
// 		"-f", "-", robLocation,
// 	}

// 	cmd := exec.Command("docker", runRobInstallerArgs...)

// 	cmd.Stdin = bytes.NewBufferString(robInstaller)

// 	cmd.Stdout = os.Stdout

// 	killChannel := make(chan os.Signal, 1)

// 	signal.Notify(killChannel,
// 		syscall.SIGHUP,
// 		syscall.SIGINT,
// 		syscall.SIGTERM,
// 		syscall.SIGQUIT,
// 	)

// 	err := cmd.Start()

// 	if err != nil {
// 		return err
// 	}

// 	go manageProcessReaping(cmd, killChannel)

// 	err = cmd.Wait()

// 	// Indicates that 'manageProcessReaping' can exit
// 	killChannel <- RJSignal{}

// 	if err != nil {
// 		return err
// 	}

// 	//Equivalent to "push therileyjohnson/{tag}"
// 	runRobPushArgs := []string{
// 		"push", fmt.Sprintf("therileyjohnson/rob:%s", tag),
// 	}

// 	cmd = exec.Command("docker", runRobPushArgs...)

// 	cmd.Stdout = os.Stdout

// 	err = cmd.Start()

// 	if err != nil {
// 		return err
// 	}

// 	go manageProcessReaping(cmd, killChannel)

// 	err = cmd.Wait()

// 	// Indicates that 'manageProcessReaping' can exit
// 	killChannel <- RJSignal{}

// 	return err
// }

func rjReapServer() error {
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

func rjUpdateSelf() error {
	runRobPullArgs := []string{
		"pull", "therileyjohnson/rob:linux-latest",
	}

	cmd := exec.Command("docker", runRobPullArgs...)

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

	if err != nil {
		return err
	}

	dockerRunName := generateID()
	robName := filepath.Base(os.Args[0])

	//Equivalent to "run --rm --name {id} rob:linux-latest"
	runRobUpgradeArgs := []string{
		"run", "--rm", "--name", dockerRunName, "rob:linux-latest",
	}

	cmd = exec.Command("docker", runRobUpgradeArgs...)

	serverExecutable, err := os.Create(robName)

	if err != nil {
		return err
	}

	cmd.Stdout = serverExecutable

	err = cmd.Start()

	if err != nil {
		return err
	}

	go manageRunReaping(dockerRunName, killChannel)

	err = cmd.Wait()

	// Indicates that 'manageRunReaping' can exit
	killChannel <- RJSignal{}

	return err
}

func oldMain() {
	var rjInfo RJInfo
	var update bool

	args := parseArguments()

	rjGlobalExists, rjLocalExists, err := preFlightCheck(args.projectRoot)

	if args.upgrade {
		if runtime.GOOS != "linux" {
			fmt.Println("Sorry, automatic upgrades are currently only supported on linux systems.")
			os.Exit(1)
		}
		fmt.Println(rjUpdateSelf())
		os.Exit(0)
	}

	if args.pushat != "" {
		// rjPushRob(args.pushat)
		os.Exit(0)
	}

	if args.flightCheck {
		fmt.Printf("RJglobal file exists: %v\nRJlocal file exists:  %v\n", rjGlobalExists, rjLocalExists)
	}

	if err != nil {
		printAndExit(err)
	}

	if args.mapDirectory != "" {
		fmt.Println(newDirMap(args.mapDirectory).StringyRoot(args.spaces))
	}

	if args.kill {
		handleServerReaping()
	}

	if args.suicide {
		rjKillClones()
	}

	if args.initialize {
		// Initialize only if asked, will not be created automatically
		rjInfo.RJGlobal, err = initializeGlobal(args.projectRoot, args.force)
	} else if rjGlobalExists {
		rjInfo.RJGlobal, err = getRjGlobal(args.projectRoot)
	} else if !rjGlobalExists {
		if !args.flightCheck && !args.suicide && !args.kill {
			fmt.Println("RJglobal must be initialized before executing any command besides '-flight', '-init', '-kill', '-map', '-pushat', '-suicide', and '-upgrade'.")
		}
		os.Exit(0)
	}

	if err != nil {
		printAndExit(err)
	}

	if args.initializeLocal {
		// RJlocal will be created automatically, however it can be remade if 'args.initializeLocal' and 'args.force' are true
		rjInfo.RJLocal, err = initializeLocal(args.projectRoot, rjInfo.RJGlobal, args.force)
	} else {
		if rjLocalExists {
			rjInfo.RJLocal, err = getRjLocal(args.projectRoot)
		} else if rjGlobalExists {
			// RJlocal will only be automatically created if an RJglobal exists
			rjInfo.RJLocal, err = initializeLocal(args.projectRoot, rjInfo.RJGlobal, args.force)
		}
	}

	if err != nil {
		printAndExit(err)
	}

	if args.token != "" {
		if _, err := os.Stat(args.token); err == nil {
			rjInfo.token, err = getGithubToken(args.tokenFilePath)
			if err != nil {
				fmt.Println(errors.Wrap(err, "could not get token from token.json"))
			}
		} else {
			rjInfo.token = args.token
		}

		rjInfo.token = args.token
	} else {
		rjInfo.token, err = getGithubToken("./token.json")
	}

	if args.local {
		if args.list {
			if args.selectProject != "" {
				index := getProjectIndex(args.selectProject, rjInfo.RJGlobal.Projects)

				if index != -1 {
					printProject(rjInfo.RJGlobal.Projects[index], rjInfo.RJLocal)
				} else {
					fmt.Println("Project specified does not exist.")
				}
			} else {
				prettyPrintStruct(rjInfo.RJLocal)
			}
		} else if args.update {
			if args.selectProject != "" {
				index := getProjectIndex(args.selectProject, rjInfo.RJGlobal.Projects)
				if index != -1 {
					projectID := rjInfo.RJGlobal.Projects[index].ID
					rjLocalProject, localProjectExists := rjInfo.RJLocal.Projects[projectID]

					if args.updateSitePath != "" {
						rjInfo.RJGlobal.Projects[index].SitePath = args.updateSitePath
						update = true
					}

					if localProjectExists && args.updateLocalPath != "" {
						rjLocalProject.Path = args.updateLocalPath
						update = true
					}

					if localProjectExists {
						rjInfo.RJLocal.Projects[projectID] = rjLocalProject
					}
				} else {
					fmt.Println("Project specified does not exist.")
				}
			} else if args.updateSearchPath != "" {
				if paths, err := parseUpdateSearchPath(args.updateSearchPath, rjInfo.RJLocal.SearchPaths); err != nil {
					fmt.Println("Could not update the local search paths with the provided input")
				} else {
					rjInfo.RJLocal.SearchPaths = paths
					update = true
				}
			} else {
				for _, rjProject := range rjInfo.RJGlobal.Projects {
					updated := 0
					if _, rjLocalExists := rjInfo.RJLocal.Projects[rjProject.ID]; !rjLocalExists {
						rjInfo.RJLocal.Projects[rjProject.ID] = RJLocalProject{}
						updated++
					}

					if updated != 0 {
						fmt.Printf("%d Projects updated from RJglobal to RJlocal!", updated)
						update = true
					} else {
						fmt.Println("Local Projects already in sync with global Projects.")
					}
				}
			}
		} else if args.add {
			if args.updateSearchPath != "" {
				rjInfo.RJLocal.SearchPaths = append(rjInfo.RJLocal.SearchPaths, args.updateSearchPath)
				update = true
			} else if args.updateLocalPath != "" {
				if args.selectProject != "" {
					index := getProjectIndex(args.selectProject, rjInfo.RJGlobal.Projects)
					if index != -1 {
						rjProject := rjInfo.RJGlobal.Projects[index]
						rjLocalProject, localProjectExists := rjInfo.RJLocal.Projects[rjProject.ID]

						if localProjectExists && args.updateLocalPath != "" {
							rjLocalProject.Path = args.updateLocalPath

							if err = writeRjTag(rjProject.ID, args.updateLocalPath); err != nil {
								fmt.Println(errors.Wrap(err, "problem automatically writing to the .RJtag file in the new project's local path"))
							}

							if err := correctPackageJSON(args.updateLocalPath, rjProject.SitePath); err != nil {
								fmt.Println(errors.Wrap(err, "problem automatically updating the package.json file in the new project's local path"))
							}

							rjInfo.RJLocal.Projects[rjProject.ID] = rjLocalProject
							update = true
						}
					} else {
						fmt.Println("Project specified does not exist.")
					}
				} else {
					fmt.Println("Cannot add the local path to a project if a project is not specified.")
				}
			} else {
				fmt.Println("Nothing specified to add locally.")
			}
		} else if args.remove {
			if args.selectProject != "" {
				if index := getProjectIndex(args.selectProject, rjInfo.RJGlobal.Projects); index != -1 {
					projectName := rjInfo.RJGlobal.Projects[index].Name
					projectID := rjInfo.RJGlobal.Projects[index].ID

					if _, localProjectExists := rjInfo.RJLocal.Projects[projectID]; localProjectExists {
						os.Remove(path.Join(rjInfo.RJLocal.Projects[projectID].Path, ".RJtag"))

						delete(rjInfo.RJLocal.Projects, projectID)
						fmt.Printf("Project successfully deleted %s locally.\n", projectName)
						update = true
					} else {
						fmt.Printf("Project %s does not exist locally.\n", projectName)
					}
				} else {
					fmt.Println("Project specified does not exist.")
				}
			} else if args.updateSearchPath != "" {
				parseList := make([]string, 0)
				if strings.Contains(args.updateSearchPath, ",") {
					parseList = strings.Split(args.updateSearchPath, ",")
				} else if strings.Contains(args.updateSearchPath, "|") {
					parseList = strings.Split(args.updateSearchPath, "|")
				} else {
					parseList = append(parseList, args.updateSearchPath)
				}

				for _, parseString := range parseList {
					index, err := strconv.ParseUint(parseString, 10, 64)
					if err == nil {
						if int(index) < len(rjInfo.RJLocal.SearchPaths) {
							rjInfo.RJLocal.SearchPaths = append(rjInfo.RJLocal.SearchPaths[:index], rjInfo.RJLocal.SearchPaths[index+1:]...)
							update = true
						}
					}
				}
			}
		} else {
			prettyPrintStruct(rjInfo.RJLocal)
		}
	} else {
		if args.list {
			if args.selectProject == "" {
				for _, project := range rjInfo.RJGlobal.Projects {
					printProject(project, rjInfo.RJLocal)
				}
			} else {
				index := getProjectIndex(args.selectProject, rjInfo.RJGlobal.Projects)

				if index != -1 {
					printProject(rjInfo.RJGlobal.Projects[index], rjInfo.RJLocal)
				} else {
					fmt.Println("Project specified does not exist.")
				}
			}
		} else if args.update {
			if args.selectProject != "" {
				index := getProjectIndex(args.selectProject, rjInfo.RJGlobal.Projects)
				if index != -1 {
					rjProject := rjInfo.RJGlobal.Projects[index]
					rjLocalProject, localProjectExists := rjInfo.RJLocal.Projects[rjProject.ID]

					if args.updateSitePath != "" {
						rjInfo.RJGlobal.Projects[index].SitePath = args.updateSitePath

						if rjLocalProject.Path != "" {
							if err := correctPackageJSON(args.updateLocalPath, rjProject.SitePath); err != nil {
								fmt.Println(errors.Wrap(err, "problem automatically updating the package.json file in the new project's local path"))
							}
						}

						update = true
					}

					if localProjectExists && args.updateLocalPath != "" {
						rjLocalProject.Path = args.updateLocalPath

						if err = writeRjTag(rjProject.ID, args.updateLocalPath); err != nil {
							fmt.Println(errors.Wrap(err, "problem automatically writing to the .RJtag file in the new project's local path"))
						}

						update = true
					}

					if localProjectExists {
						rjInfo.RJLocal.Projects[rjProject.ID] = rjLocalProject
					}
				} else {
					fmt.Println("Project specified does not exist.")
				}
			} else {
				if rjInfo.token != "" {
					for index, rjProject := range rjInfo.RJGlobal.Projects {
						var newDescription string
						newDescription, err = getProjectDescription(rjProject.Name, rjInfo.token)

						if err != nil {
							fmt.Println(errors.Wrapf(err, "problem fetching description for Project '%s'", rjProject.Name))
						} else {
							if newDescription != rjProject.Description {
								rjInfo.RJGlobal.Projects[index].Description = newDescription
								fmt.Printf("Description for Project '%s' successfully updated.", rjProject.Name)
								update = true
							}
						}
					}

					if err == nil && !update {
						fmt.Println("Descriptions for all projects are already up to date.")
					}
				} else {
					fmt.Println("Cannot update project descriptions without Github API token")
				}
			}
		} else if args.add {
			// Handles cases where a local command is meant to be executed but the '-l' flag is neglected
			if args.updateSearchPath != "" || (args.selectProject == "" && args.updateLocalPath != "") {
				fmt.Println("Don't forget to add the '-l' flag if you want to adjust the local search path or update the local path of a project.")
			} else {
				if args.selectProject != "" {
					if !checkProjectExistance(args.selectProject, rjInfo.RJGlobal.Projects) {
						if _, err := url.ParseRequestURI(args.selectProject); err == nil {
							projectID := generateID()

							rjProject := RJProject{
								ID:       projectID,
								Name:     path.Base(args.selectProject),
								SitePath: path.Join("./projects/", path.Base(args.selectProject)),
								URL:      args.selectProject,
							}

							os.MkdirAll(rjProject.SitePath, os.ModePerm)

							rjProject.Description, err = getProjectDescription(rjProject.Name, rjInfo.token)

							if err != nil {
								fmt.Println("Could not fetch description, defaulting to blank.")
								rjProject.Description = ""
							}

							rjInfo.RJGlobal.Projects = append(rjInfo.RJGlobal.Projects, rjProject)
							rjInfo.RJLocal.Projects[projectID] = RJLocalProject{Path: args.updateLocalPath}

							if args.updateLocalPath != "" {
								if err = writeRjTag(projectID, args.updateLocalPath); err != nil {
									fmt.Println(errors.Wrap(err, "problem automatically writing to the .RJtag file in the new project's local path"))
								}
							}

							update = true
						} else {
							fmt.Println("Project URL is not valid.")
						}
					} else {
						fmt.Println("Project with that URL already exists.")
					}
				} else {
					fmt.Println("Cannot add a Project when there is no URL specified.")
				}
			}
		} else if args.remove {
			if index := getProjectIndex(args.selectProject, rjInfo.RJGlobal.Projects); index != -1 {
				projectName := rjInfo.RJGlobal.Projects[index].Name

				_, localProjectExists := rjInfo.RJLocal.Projects[rjInfo.RJGlobal.Projects[index].ID]

				if localProjectExists {
					delete(rjInfo.RJLocal.Projects, rjInfo.RJGlobal.Projects[index].ID)
				}

				if args.local {
					if localProjectExists {
						fmt.Println("Project successfully deleted locally.")
					} else {
						fmt.Println("Project specified does not exist locally.")
					}
				} else {
					rjInfo.RJGlobal.Projects = append(rjInfo.RJGlobal.Projects[:index], rjInfo.RJGlobal.Projects[index+1:]...)
					pruneLocal(&rjInfo)

					fmt.Printf("Deleted Project %s\n", projectName)
					update = true
				}
			} else {
				fmt.Println("Project specified does not exist.")
			}
		}
	}

	if args.prune {
		update = rjPrune(&rjInfo)
	}

	if args.discover {
		update = rjDiscover(&rjInfo, &args)
	}

	if args.syncronizeLocal {
		if args.selectProject == "" {
			for _, rjProject := range rjInfo.RJGlobal.Projects {
				handleSyncronizeLocal(&rjProject, &rjInfo.RJLocal)
			}
		} else {
			if index := getProjectIndex(args.selectProject, rjInfo.RJGlobal.Projects); index != -1 {
				rjProject := rjInfo.RJGlobal.Projects[index]

				handleSyncronizeLocal(&rjProject, &rjInfo.RJLocal)
			} else {
				fmt.Println("Project specified does not exist.")
			}
		}
	}

	if args.clone {
		if args.selectProject == "" {
			for _, rjProject := range rjInfo.RJGlobal.Projects {
				handleCloneProject(&rjProject, &rjInfo.RJLocal, args.force)
			}
		} else {
			if index := getProjectIndex(args.selectProject, rjInfo.RJGlobal.Projects); index != -1 {
				rjProject := rjInfo.RJGlobal.Projects[index]
				handleCloneProject(&rjProject, &rjInfo.RJLocal, args.force)
			} else {
				fmt.Println("Project specified does not exist.")
			}
		}
	}

	if args.build {
		if args.root {
			// update = handleBuildRoot(&rjInfo, &args)
		} else {
			if args.selectProject == "" {
				for _, rjProject := range rjInfo.RJGlobal.Projects {
					if rjBuild(&rjInfo, &rjProject, &args) {
						// Can't do update = rjBuild(...) because of possible false result
						update = true
					}
				}
			} else {
				if index := getProjectIndex(args.selectProject, rjInfo.RJGlobal.Projects); index != -1 {
					update = rjBuild(&rjInfo, &rjInfo.RJGlobal.Projects[index], &args)
				} else {
					fmt.Println("Project specified does not exist.")
				}
			}
		}
	}

	if args.run {
		handleProjectRun(&rjInfo, &args)
	}

	if update {
		err := writeUpdate(args.projectRoot, rjInfo)

		if err != nil {
			printAndExit(errors.Wrap(err, "problem writing out updated information to file"))
		}
	}
}
