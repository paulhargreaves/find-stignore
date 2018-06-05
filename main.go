// version 1.0 - Paul Hargreaves
//
// A syncthing '.stignore' version of find. It only outputs what is missing.
//
// Note: Can only run on the target system (so, localhost for syncthing). This is because there is no syncthing API
// to browse folders and find files in them.
// If this changes, search for instances of os.pathseparator and replace them, change the walk code for the localFS to the API,
// and the rest of the code should pretty much work as-is. Or will all be replaced by something better ;-)



package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"reflect"
	"sort"
	"strings"
)

const (
	stversions = ".stversions"
	stignore   = ".stignore"
)

func main() {
	hostURLp := flag.String("url", "http://localhost:8384", "The host URL. Do not attempt to use this with a 'remote' Syncthing server since it expects to see the local filesystem that matches what syncthing sees.")
	apiKeyp := flag.String("apikey", "", "The API key that the Syncthing gui shows. (Required)")
	folderIDp := flag.String("folderid", "", "The folder ID that you want to check for ignores. See the Syncthing GUI. (Required)")
	print0p := flag.Bool("print0", false, "Output in a format suitable for tools like xargs by null (zero) terminating lines. This means that files with newlines will be correctly listed.")
	dirsOnlyp := flag.Bool("dirsonly", false, "Only output directories, not files")
	filesOnlyp := flag.Bool("filesonly", false, "Only output files, not directories")
	showSyncthingp := flag.Bool("showallconfig", false, "Show all config files (e.g. .stfolder, .stversions) Do NOT use the output with this enabled unless you are sure what you are looking at.")
	//reversep := flag.Bool("reverse", false, "Debug option - used to switch around the local list vs. the syncthing list. Do not use for anything other than testing.")
	flag.Parse()

	hostURL := *hostURLp
	apiKey := *apiKeyp
	folderID := *folderIDp
	print0 := *print0p
	dirsOnly := *dirsOnlyp
	filesOnly := *filesOnlyp
	showSyncthing := *showSyncthingp
	//reverse := *reversep

	if apiKey == "" {
		fmt.Fprintf(os.Stderr, "Error: Missing --apikey. See the Syncthing GUI.\n")
		os.Exit(1)
	}
	if folderID == "" {
		fmt.Fprintf(os.Stderr, "Error: Missing --folderid. See the Syncthing GUI, some folders use the name unless there is a specific folderid.\n")
		os.Exit(1)
	}
	if dirsOnly && filesOnly {
		fmt.Fprintf(os.Stderr, "Error: Both dirsonly and filesonly are set. Choose one.\n")
		os.Exit(1)
	}
	/*if reverse {
		fmt.Fprintf(os.Stderr, "WARNING: Reverse option set. Do not use these results for anything serious!\n")
	}*/

	//
	folderPath, folderMarker, folderVersions := getConfig(hostURL, apiKey, folderID)
	if folderVersions == "" {
		folderVersions = stversions
	}

	// Remove the path separator if it's the last character, since the UI allowed us to have both types and doesn't attempt to normalise
	if folderPath[len(folderPath)-1] == os.PathSeparator {
		folderPath = folderPath[:len(folderPath)-1]
	}

	browseDecode := getJSONFromHTML("GET", hostURL+"/rest/db/browse?folder="+folderID, apiKey)

	// Start running through all the db view of the directories
	var fromSyncthing map[string]bool // string is the path/file/dir, bool is true if dir, false if not
	fromSyncthing = make(map[string]bool)
	processDBBrowseDirectory(folderPath, browseDecode, fromSyncthing)

	// Start running through the system view of the directories
	// We can't use the rest/system/browse uri since it only returns directories
	var fromLocalFS map[string]bool // string is the path/file/dir, bool is true if dir, false if not
	fromLocalFS = make(map[string]bool)
	processSystemBrowseDirectory(folderPath, fromLocalFS)

	// Debug: Reversing the lists?
	/*if reverse {
		tempFromLocalFS := fromLocalFS
		fromLocalFS = fromSyncthing
		fromSyncthing = tempFromLocalFS
	}*/

	// Remove all syncthing objects, unless the user has requested them
	if !showSyncthing {
		// Now we need to delete the .stfolder entry
		folderMarkerPath := folderPath + string(os.PathSeparator) + folderMarker
		if _, ok := fromLocalFS[folderMarkerPath]; !ok {
			log.Fatal("Folder marker ", folderMarkerPath, " specified but not found in the syncthing output.")
		}

		delete(fromLocalFS, folderMarkerPath)

		// Now the .stignore
		stignorePath := folderPath + string(os.PathSeparator) + stignore
		delete(fromLocalFS, stignorePath)

		// Now the .stversions, if they exist
		// We could do this in the local walker (and it would likely be easier), but if we do then we can't use the reverse debug option
		removeVersions(fromLocalFS, folderPath, folderVersions)
	}

	// Figure out the sorted list of file names
	var sortedKeysLocalFS []string
	for k := range fromLocalFS {
		sortedKeysLocalFS = append(sortedKeysLocalFS, k)
	}
	sort.Strings(sortedKeysLocalFS)

	// Now read the localfs list and output any that are missing in the syncthing list - these will be ones that are ignored
	for _, v := range sortedKeysLocalFS {

		// We only want dirs?
		if dirsOnly && fromLocalFS[v] == false {
			continue // skip, it's a file
		}

		// We only want files?
		if filesOnly && fromLocalFS[v] == true {
			continue // skip, it's a directory
		}

		// Now check if found
		if _, found := fromSyncthing[v]; !found {
			if !print0 {
				fmt.Println(v)
			} else { // print0
				fmt.Print(v + string('\000'))
			}
		}
	}
}

// theFS - which list to process
// folderPath - where the root of the folder is
// versions -
func removeVersions(theFS map[string]bool, folderPath string, versions string) {
	// First need to work out if the versions is a full path or not.
	excludeVersions := versions
	if !strings.ContainsRune(versions, os.PathSeparator) {
		// Doesn't look like a path so we'll just use the folderpath
		excludeVersions = folderPath + string(os.PathSeparator) + versions
	}

	// Shouldn't happen... but anyway...
	if len(excludeVersions) < 2 {
		panic("Did someone use something like / as the full versions path?")
	}

	// Remove the final pathseparator if set
	if strings.HasSuffix(excludeVersions, string(os.PathSeparator)) {
		excludeVersions = excludeVersions[:len(excludeVersions)-1]
	}

	// Mark the folder for deletion
	delete(theFS, excludeVersions)

	// Now we find any and all entries in the folder and delete those too
	excludeVersions = excludeVersions + string(os.PathSeparator)
	for k := range theFS {
		if strings.HasPrefix(k, excludeVersions) {
			delete(theFS, k)
		}
	}
}

// This recurses
// This walks a filesystem (currentDir) and adds to localFS map all the dirs [dirname]true, and files [filename]false 
func processSystemBrowseDirectory(currentDir string, localFS map[string]bool) {
	files, err := ioutil.ReadDir(currentDir)
	if err != nil {
		log.Fatal(err)
	}

	_, exists := localFS[currentDir]
	if exists {
		panic("Attempt to add entry to map again " + currentDir)
	}
	localFS[currentDir] = true // store the directory

	// Now recurse through the list of entries
	for _, file := range files {

		fullname := currentDir + string(os.PathSeparator) + file.Name()
		if file.IsDir() {
			processSystemBrowseDirectory(fullname, localFS) // Recurse
		} else { // A file
			localFS[fullname] = false // store the file
		}
	}
}

// Returns the folder path for a given folder id, as syncthing sees, e.g. "/media/foo/bar"
// Also returns the folder marker (typically .stfolder) and the versionPath for staggered file versions
// folderID is whatever syncthing expects, e.g iwjef-efw or somewords, depending on what syncthing shows in the UI
// hostEndpoint is the host:port (typically localhost:8384)
// apiKey is the API key that syncthing expects, if set
func getConfig(hostEndpoint string, apiKey string, folderID string) (string, string, string) {
	configDecode := getJSONFromHTML("GET", hostEndpoint+"/rest/system/config", apiKey)
	folderPath := ""
	folderMarker := ""
	folderVersions := ""
	for k, v := range configDecode {
		if k == "folders" {
			//fmt.Fprintf(os.Stderr, "+++ %s (%+v)\n", k, v)
			for _, b := range v.([]interface{}) {
				folderTags := b.(map[string]interface{})
				if folderTags["id"] == folderID {
					folderPath = folderTags["path"].(string)
					folderMarker = folderTags["markerName"].(string)
					// Sigh...
					if _, exists := folderTags["versioning"]; exists {
						versioningTags := folderTags["versioning"].(map[string]interface{})
						if _, exists := versioningTags["params"]; exists {
							paramsTags := versioningTags["params"].(map[string]interface{})
							if vers, exists := paramsTags["versionsPath"]; exists {
								folderVersions = vers.(string)
							}
						}
					}
					//fmt.Fprintf(os.Stderr, "+++ (%+v)\n", folderTags)
				}
			}
		}
	}
	if folderPath == "" {
		log.Fatal("No folder path found. The ID is probably incorrect.")
	}
	if folderMarker == "" {
		panic("No folder marker tag found.")
	}
	return folderPath, folderMarker, folderVersions
}

/*
// Returns the pathSeparator, e.g / or \
// hostEndpoint is the host:port (typically localhost:8384)
// apiKey is the API key that syncthing expects, if set
func getPathSeparator(hostEndpoint string, apiKey string) string {
	statusDecode := getJSONFromHTML("GET", hostEndpoint+"/rest/system/status", apiKey)

	pathSeparator := statusDecode["pathSeparator"].(string)
	if len(pathSeparator) == 0 {
		panic("No valid path separator found.")
	}
	return pathSeparator
}
*/

// This recurses
// Process a directory, and all of it's subdirectories, using the dirContents which are obtained from getJSONFromHTML
// currentDir is the 'starting' point of whatver dirContents points to
func processDBBrowseDirectory(currentDir string, dirContents map[string]interface{},
	allKnownPaths map[string]bool) {

	_, exists := allKnownPaths[currentDir]
	if exists {
		panic("Attempt to add entry to map again " + currentDir)
	}
	allKnownPaths[currentDir] = true // Store the directory

	// Now iterate through the contents of the directory
	for k, v := range dirContents {
		fullname := currentDir + string(os.PathSeparator) + k
		if reflect.ValueOf(v).Kind() == reflect.Map {
			processDBBrowseDirectory(fullname, v.(map[string]interface{}),
				allKnownPaths) // RECURSE
		} else { // A file
			allKnownPaths[fullname] = false // Store the file
		}
	}
}

// Get usable JSON content from a web serever
// requestType is going to be GET or PUT (and, likely only GET for this program)
// requestURL is the full URL we want to contact. Typically will be localhost:8384
// requestAPIKey is the API key that syncthing expects, if set
// Returns an map interface
// Fatals if the json from the html cannot be processed.
func getJSONFromHTML(requestType string, requestURL string, requestAPIKey string) map[string]interface{} {
	htmlResult := makeHTMLRequest(requestType, requestURL, requestAPIKey)

	var htmlDecode map[string]interface{}
	jsonErr := json.Unmarshal(htmlResult, &htmlDecode)
	if jsonErr != nil {
		fmt.Fprintf(os.Stderr, "%s\n", htmlResult) // Let the user figure out what on earth the output was, hopefully an error...
		log.Fatal(jsonErr)
	}

	return htmlDecode
}

// Send a request to a web endpoint
// requestType is going to be GET or PUT (and, likely only GET for this program)
// requestURL is the full URL we want to contact. Typically will be localhost:8384
// requestAPIKey is the API key that syncthing expects, if set
// Returns a list of bytes, whatever the server responds with.
// Will fatal if there is an issue.
func makeHTMLRequest(requestType string, requestURL string, requestAPIKey string) []byte {
	client := &http.Client{
		//CheckRedirect: redirectPolicyFunc,
	}
	req, err := http.NewRequest(requestType, requestURL, nil)
	if requestAPIKey != "" {
		req.Header.Add("X-API-Key", requestAPIKey)
	}
	res, err := client.Do(req)

	if err != nil {
		log.Fatal(err)
	}
	defer res.Body.Close()

	theResult, err := ioutil.ReadAll(res.Body)
	if err != nil {
		log.Fatal(err)
	}
	return theResult
}
