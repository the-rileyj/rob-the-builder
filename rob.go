package rob

type Project struct {
	ID      string `json:"id,omitempty"`
	path    string
	SiteDir string `json:"siteDir"`
	Tag     string `json:"tag"`
	URL     string `json:"url"`
}

type RjLocal struct {
	SearchDir    string            `json:"searchDir,omitempty"`
	ProjectPaths map[string]string `json:"projectPaths,omitempty"`
}

type RjConfig struct {
	Projects []Project `json:"projects"`
	rjLocal  RjLocal   //.rjlocal.json file has mapping of {searchDir:string, id:path} locally, not including in github repo
}

func main() {
	// Args:
	// Add: Adds project to rjconfig.json provided that the rjconfig.json file already
	//     exists, and both the SiteDir and Url command line args are provided
	// Build: Iterate through the projects and build them with the Node image, attaching
	//     volumes "{{Project.path}}:/usr/src/app" for the config files required to
	//     determine build context, and "{{Project.SiteDir}}:/usr/src/build" to capture
	//     the build output to the site directory
	// Discover: Try iterating through the projects and finding matching IDs, otherwise
	//     start in RjConfig.SearchDir and seek out a .rjtag file which matches any of the
	//     IDs which were not found
	// Generate: Iterate through all of the projects and generate IDs for the projects which
	//     have paths not containing an .rjtag file containing a matching id,
	// InitLocal: Initialize an empty .rjlocal.json file in the site root if it does not exist
	// Install: Bool which indicates that the dependencies for each project should be
	//     installed prior to building
	// Path: Identify the path for a project via the format "ID:path" or "Tag:path"; can be
	//     setup in rjconfig.json manually
	// Pull: Bool which indicates that the directory should be pulled from the URL for
	//     building if no path is found or specified
	// SiteDir: Modify or add a new site directory to a project, in the format "ID:SiteDir"
	//     or "Tag:SiteDir" if modifying an existing site directory for a project, or just
	//     "SiteDir" if initializing a new project with command line arg Add; can be setup
	//     in rjconfig.json manually
	// Tag: Tag an ID with an identifying string, in the format "ID:Tag"; can be setup in
	//     rjconfig.json manually
	// Url: Modify or add a new github URL to a project, in the format "ID:URL" or "Tag:URL"
	//     if modifying an existing site directory for a project, or just "URL" if
	//     initializing a new project with command line arg Add; can be setup in
	//     rjconfig.json manually
}
