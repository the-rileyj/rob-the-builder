package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	git "gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/storage/memory"
)

//==================
// Local Use Structs

// RJInfo is for centralizing global and local information for the program's use
type RJInfo struct {
	RJGlobal
	RJLocal
	token string
}

// RJGlobal is for storing global information about projects, committed
type RJGlobal struct {
	Projects []RJProject `json:"projects"`
}

// RJLocalProject is for storing local information about a given project, not committed
type RJLocalProject struct {
	CurrentProjectHash, Path, LastBuildHash string
}

// RJProject is for storing global information about a given project, committed
type RJProject struct {
	Description string `json:"description"`
	ID          string `json:"id"`
	Name        string `json:"name"`
	SitePath    string `json:"sitePath"`
	URL         string `json:"url"`
}

// RJLocal is for storing local information about projects and where to start searching for local projects, not committed
type RJLocal struct {
	Projects    map[string]RJLocalProject `json:"projects"`
	SearchPaths []string                  `json:"searchPaths"`
}

type arguments struct {
	selectProject, projectRoot, token, tokenFilePath, updateSearchPath, updateSitePath, updateLocalPath                                           string
	add, clone, discover, flightCheck, force, initialize, initializeLocal, list, local, prune, syncronizeLocal, remove, update, updateDescription bool
}

type githubToken struct {
	Token string `json:"token"`
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
// File Crawler Data Structure

type directorySearch struct {
	directoryPaths []string
	searchPaths    []string
}

func checkDirectoryForSearchFile(searchFile, dirname string, pathChan chan directorySearch) {
	returnDirectorySearched := directorySearch{make([]string, 0), make([]string, 0)}
	f, err := os.Open(dirname)

	if err != nil {
		pathChan <- directorySearch{}
		return
	}

	foundPaths, err := f.Readdir(-1)
	f.Close()

	if err != nil {
		pathChan <- directorySearch{}
		return
	}

	for _, foundPath := range foundPaths {
		if foundPath.IsDir() {
			returnDirectorySearched.directoryPaths = append(returnDirectorySearched.directoryPaths, path.Join(dirname, foundPath.Name()))
		} else if foundPath.Name() == searchFile {
			returnDirectorySearched.searchPaths = append(returnDirectorySearched.searchPaths, path.Join(dirname, foundPath.Name()))
		}
	}

	pathChan <- returnDirectorySearched
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

func fileSearcher(findFile, rootPath string, maxChanNumber int) []string {
	if maxChanNumber < 1 {
		maxChanNumber = maxParallelism()
	}

	currentChanNumber := 0
	pathChan := make(chan directorySearch)

	checkPaths := []string{rootPath}
	foundPaths := make([]string, 0)

	for len(checkPaths) != 0 || currentChanNumber != 0 {
		if len(checkPaths) != 0 && currentChanNumber != maxChanNumber {
			checkPath := checkPaths[0]
			checkPaths = append(checkPaths[:0], checkPaths[1:]...)

			go checkDirectoryForSearchFile(findFile, checkPath, pathChan)
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
	buffer.WriteRune('r')
	for index := 0; index < 9; index++ {
		buffer.WriteString(fmt.Sprintf("%d", rand.Int()%10))
	}
	buffer.WriteRune('j')
	return buffer.String()
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

	rjLocal := RJLocal{make(map[string]RJLocalProject), make([]string, 0)}

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

func maxParallelism() int {
	maxProcs := runtime.GOMAXPROCS(0)
	numCPU := runtime.NumCPU()
	if maxProcs < numCPU {
		return maxProcs
	}
	return numCPU
}

func parseArguments() arguments {
	var args arguments
	flag.StringVar(&args.tokenFilePath, "t", "./token.json", "Name of the json file in the project root with the gitlab token for gathering the project descriptions, or the token directly.")
	flag.StringVar(&args.projectRoot, "rp", "./", "Path to the project root.")
	flag.StringVar(&args.selectProject, "p", "", "Project selection for creating, reading, updating, and deleting; if creating this should be the Github URL of project you want to add.")
	flag.StringVar(&args.updateSearchPath, "sep", "", "The path where the auto discovery for directories with RJtag's should start searching. No-op if not used in conjunction with 'local' command line argument, as well as 'add', 'up', or 'rm'.")
	flag.StringVar(&args.updateSitePath, "sip", "", "Path relative to the root of the rj website project, where the project should be output to when built. No-op if not used in conjunction with 'up' command line argument.")
	flag.StringVar(&args.updateLocalPath, "lp", "", "Path on the machine, where the project should be built from. No-op if not used in conjunction with 'up' command line argument.")

	flag.BoolVar(&args.add, "add", false, "Adds the project specified by 'project' with information provided via command line args 'description', site-path', and 'local-path'.")
	flag.BoolVar(&args.clone, "clone", false, "Clones the local project specified by 'project' if it does not have a local hash or all local projects which do not have a local hash if 'project' is not specified; NOTE: this will clear the whole directory prior to cloning.")
	flag.BoolVar(&args.updateDescription, "desc", false, "Used in conjuction with 'up', update the descriptions for the projects.")
	flag.BoolVar(&args.discover, "discover", false, "Search for RJtag's correlating to the ID's of the projects in RJglobal, starting in 'SearchPath' specified in the RJlocal file.")
	flag.BoolVar(&args.force, "force", false, "Force the operation(s); be careful.")
	flag.BoolVar(&args.initialize, "init", false, "Initialize a new RJglobal file if not already created. Can be forced.")
	flag.BoolVar(&args.initializeLocal, "init-local", false, "Initialize a new RJglobal file if not already created. Can be forced.")
	flag.BoolVar(&args.flightCheck, "flight", false, "Prints the output of the 'pre-flight check'.")
	flag.BoolVar(&args.local, "l", false, "Modifies the behaviour of some commands to only have affects on the RJlocal config.")
	flag.BoolVar(&args.list, "ls", false, "Lists the details of the project specified by 'project', or all projects if none are specified or found, as pretty printed JSON.")
	flag.BoolVar(&args.prune, "prune", false, "Deletes all local projects not found in RJglobal.")
	flag.BoolVar(&args.syncronizeLocal, "sync", false, "Checks the local git hash against what's in the remote repo and updates either the local project specified by 'project' or all local projects if 'project' is not specified.")
	flag.BoolVar(&args.remove, "rm", false, "Removes the project specified by 'project', adding the 'local' flag will only delete it locally..")
	flag.BoolVar(&args.update, "up", false, "Updates the project specified by 'project' with information provided via command line args 'description', site-path', and 'local-path'.")

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

func pruneLocal(projects []RJProject, localProjects map[string]RJLocalProject) map[string]RJLocalProject {
	configProjects := make(map[string]bool)
	for _, project := range projects {
		configProjects[project.ID] = true
	}

	returnProjects := make(map[string]RJLocalProject)

	for projectID := range localProjects {
		if configProjects[projectID] {
			returnProjects[projectID] = localProjects[projectID]
		}
	}

	return returnProjects
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

func main() {
	var rjInfo RJInfo
	var update bool

	args := parseArguments()

	rjGlobalExists, rjLocalExists, err := preFlightCheck(args.projectRoot)

	if args.flightCheck {
		fmt.Printf("RJglobal file exists: %v\nRJlocal file exists:  %v\n", rjGlobalExists, rjLocalExists)
	}

	if err != nil {
		printAndExit(err)
	}

	if args.initialize {
		// Initialize only if asked, will not be created automatically
		rjInfo.RJGlobal, err = initializeGlobal(args.projectRoot, args.force)
	} else if rjGlobalExists {
		rjInfo.RJGlobal, err = getRjGlobal(args.projectRoot)
	} else if !rjGlobalExists {
		if !args.flightCheck {
			fmt.Println("RJglobal must be initialized before executing any command besides 'flight' and 'init'.")
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
			}
		} else if args.add {
			if args.updateSearchPath != "" {
				rjInfo.RJLocal.SearchPaths = append(rjInfo.RJLocal.SearchPaths, args.updateSearchPath)
				update = true
			} else if args.updateLocalPath != "" {
				if args.selectProject != "" {
					index := getProjectIndex(args.selectProject, rjInfo.RJGlobal.Projects)
					if index != -1 {
						projectID := rjInfo.RJGlobal.Projects[index].ID
						rjLocalProject, localProjectExists := rjInfo.RJLocal.Projects[projectID]

						if localProjectExists && args.updateLocalPath != "" {
							rjLocalProject.Path = args.updateLocalPath

							if err = writeRjTag(projectID, args.updateLocalPath); err != nil {
								fmt.Println(errors.Wrap(err, "problem automatically writing to the .RJtag file in the new project's local path"))
							}

							update = true
						}

						if localProjectExists {
							rjInfo.RJLocal.Projects[projectID] = rjLocalProject
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
					projectID := rjInfo.RJGlobal.Projects[index].ID
					rjLocalProject, localProjectExists := rjInfo.RJLocal.Projects[projectID]

					if args.updateSitePath != "" {
						rjInfo.RJGlobal.Projects[index].SitePath = args.updateSitePath
						update = true
					}

					if localProjectExists && args.updateLocalPath != "" {
						rjLocalProject.Path = args.updateLocalPath

						if err = writeRjTag(projectID, args.updateLocalPath); err != nil {
							fmt.Println(errors.Wrap(err, "problem automatically writing to the .RJtag file in the new project's local path"))
						}

						update = true
					}

					if localProjectExists {
						rjInfo.RJLocal.Projects[projectID] = rjLocalProject
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
			if args.updateSearchPath != "" || args.updateLocalPath != "" {
				fmt.Println("Don't forget to add the '-l' flag if you want to adjust the local search path or update the local path of a project.")
			} else {
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

					fmt.Printf("Deleted Project %s\n", projectName)
					update = true
				}
			} else {
				fmt.Println("Project specified does not exist.")
			}
		}
	}

	if args.prune {
		localProjectsBefore := len(rjInfo.RJLocal.Projects)
		rjInfo.RJLocal.Projects = pruneLocal(rjInfo.RJGlobal.Projects, rjInfo.RJLocal.Projects)
		localProjectsAfter := len(rjInfo.RJLocal.Projects)

		if localProjectsAfter == localProjectsBefore {
			fmt.Println("No Local Projects Pruned.")
		} else {
			fmt.Printf("%d Local Projects Pruned.\n", localProjectsBefore-localProjectsAfter)
			update = true
		}
	}

	if args.discover {
		if len(rjInfo.RJLocal.SearchPaths) != 0 {
			discoveredTagPaths := make([]string, 0)

			for _, searchPath := range rjInfo.RJLocal.SearchPaths {
				discoveredTagPaths = append(discoveredTagPaths, fileSearcher(".RJtag", searchPath, -1)...)
			}

			for _, discoveredTagPath := range discoveredTagPaths {
				if id, err := ioutil.ReadFile(discoveredTagPath); err != nil {
					fmt.Println(errors.Wrapf(err, "could not get id from tag at %s", discoveredTagPath))
				} else {
					rjInfo.RJLocal.Projects[string(id)] = RJLocalProject{Path: filepath.Dir(discoveredTagPath)}
				}
			}

			rjInfo.RJLocal.Projects = pruneLocal(rjInfo.RJGlobal.Projects, rjInfo.RJLocal.Projects)

			update = true
		} else {
			fmt.Println("Cannot discover if the list of paths to search in is empty (RJlocal.SearchPaths).")
		}
	}

	if args.syncronizeLocal {
		if args.selectProject == "" {
			for _, rjProject := range rjInfo.RJGlobal.Projects {
				if rjLocalProject, rjLocalProjectExists := rjInfo.RJLocal.Projects[rjProject.ID]; rjLocalProjectExists && rjLocalProject.Path != "" {
					if rjLocalProject.Path != "" {
						if newlySynced, err := syncronizeLocal(rjProject, rjLocalProject); err == nil {
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
		} else {
			if index := getProjectIndex(args.selectProject, rjInfo.RJGlobal.Projects); index != -1 {
				rjProject := rjInfo.RJGlobal.Projects[index]
				if rjLocalProject, rjLocalProjectExists := rjInfo.RJLocal.Projects[rjProject.ID]; rjLocalProjectExists && rjLocalProject.Path != "" {
					if rjLocalProject.Path != "" {
						if newlySynced, err := syncronizeLocal(rjProject, rjLocalProject); err != nil {
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
			} else {
				fmt.Println("Project specified does not exist.")
			}
		}
	}

	if args.clone {
		if args.selectProject == "" {
			for _, rjProject := range rjInfo.RJGlobal.Projects {
				if rjLocalProject, rjLocalProjectExists := rjInfo.RJLocal.Projects[rjProject.ID]; rjLocalProjectExists && rjLocalProject.Path != "" {
					if rjLocalProject.Path != "" {
						if _, err := getLocalProjectCommit(rjLocalProject.Path); err == nil {
							if args.force {
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
		} else {
			if index := getProjectIndex(args.selectProject, rjInfo.RJGlobal.Projects); index != -1 {
				rjProject := rjInfo.RJGlobal.Projects[index]
				if rjLocalProject, rjLocalProjectExists := rjInfo.RJLocal.Projects[rjProject.ID]; rjLocalProjectExists && rjLocalProject.Path != "" {
					if rjLocalProject.Path != "" {
						if _, err := getLocalProjectCommit(rjLocalProject.Path); err == nil {
							if args.force {
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
			} else {
				fmt.Println("Project specified does not exist.")
			}
		}
	}

	if update {
		err := writeUpdate(args.projectRoot, rjInfo)

		if err != nil {
			printAndExit(errors.Wrap(err, "problem writing out updated information to file"))
		}
	}
}
