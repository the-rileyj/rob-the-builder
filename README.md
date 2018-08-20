# ROB - React Outline Builder

## ROB the Builder

Build tool for React projects in RJ's site (Work In Progess)

## Overview

**Note**: ROB is transitioning from the stdlib argument parser to [Cobra](https://github.com/spf13/cobra), the readme has been updated to reflect the coming changes

ROB is a tool used for tagging react projects which are included in some manner in [RJ's site](https://github.com/the-rileyj/RJ-Go-Site-V2). Though this project is pretty much solely for my personal use, instructions will still be provided because this project has gotten some attention (albeit minor) from people who are not me.

## Usage Explantion

ROB was born after doing some work on my back-end and realizing that integration with React SPA's was going to involve a lot of fanangling on my part. I would need to keep the versions of the web apps consistant on my website, but to do that would require a lot of manual building and rebuilding as new versions came to be. Cloning the web apps directly would leave a lot of files needed for cleanup on top of the aforementioned management. I didn't to create shell scripts individually for each project for building and cleanup. Thus ROB came to fruition to solve those issues, among others including building the web server within a container (removing the need to install Golang), building the webapps in a container (removes the need to install Node/NPM), managing automatic upgrades for the server on new commits, and overall serve as a tool for managing my server and assisting in local development of the web server.

## General Workflow

Using ROB in a project typically goes like this:

1. **Initalize RJglobal.json file**

   You need an `RJglobal.json` file in the root of the project you're going to be building the *React* projects to, the `RJglobal.json` file will keep track of the projects and their information.

   ```bash
   rj:~/RJsite$ rob init
   ```

   Or

   ```bash
   rj:~/$ rob init -r "./path/to/RJsite"
   ```

2. **Add project to RJglobal**

   A project is added via it's github URL, ***all** other information for the project is either derived from the github project URL or handled internally (ID generation). The project name comes from the URL (ex. https://github.com/the-rileyj/rj-internship-2018 would get the name rj-internship-2018), the site path (path relative to the root project (again, which is usually [RJ's site](https://github.com/the-rileyj/RJ-Go-Site-V2))) is automatically generated as `./projects/PROJECT_NAME`, and an ID is automatically generated.

   ```bash
   rj:~/RJsite$ rob add project "https://github.com/the-rileyj/rj-internship-2018"
   ```

   The latter command will add the following information to the projects field of `RJglobal.json`:

   ```javascript
   {
    "description": "React Spectacle Presentation on RJ's 2018 Summer Internship",
    "id": "r011170886j",
    "name": "rj-internship-2018",
    "sitePath": "projects/rj-internship-2018",
    "url": "https://github.com/the-rileyj/rj-internship-2018"
   }
   ```

   ***NOTE:**

   For the **description** to be automatically grabbed from github, you must generate a Personal Access Token on github with the `public_repo` scope, then provide the token when adding the project through the `-t` arg or provide the path to a `*.json` file with a token field which has the token there.

   ![public_repo scope][public_repo_scope]

   Adding token through command line:

   ```bash
   rj:~/RJsite$ rob add project https://github.com/the-rileyj/rj-internship-2018 -t "github PAC Token"
   ```

   Adding token via `*.json` file

   ```bash
   rj:~/RJsite$ rob add project https://github.com/the-rileyj/rj-internship-2018 -t "path/to/token/json/token.json" ...
   ```

   Token JSON file schema:

   ```javascript
   {
    "token": "token string"
   }
   ```

   **NOTE:**

   The Project is automatically added to the `RJlocal.json` file, but none of the fields are filled in.

   The command, in addition to the information added to `RJglobal.json` as shown above,  will add the following information to the projects object of `RJlocal.json`:

   ```javascript
   {
    "projects": {
        "r011170886j": {
            "CurrentProjectHash": "",
            "Path": "",
            "LastBuildHash": ""
        }
    },
    "searchPaths": []
   }
   ```

3. **Add Information to a Local Project**

   If you're planning to build your projects from containers (which is sometimes desirable in scenarios like deployment)

   **NOTE:**

   As previously mentioned, though this step is not nessesary for building, it is most likely desired if testing and working on a  *React* project locally because building from the github repo can take a substantial amount of time.

**MORE SOON**


## General Usage

