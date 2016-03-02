package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/cloudfoundry/cli/plugin"
	"github.com/ibmjstart/bluemix-cloudant-sync/CloudantAccountModel"
	"github.com/ibmjstart/bluemix-cloudant-sync/prompts"
	"github.com/ibmjstart/bluemix-cloudant-sync/utils"
	"io/ioutil"
	"net/http"
	"strings"
	"time"
)

var ENDPOINTS = []string{"https://api.ng.bluemix.net", "https://api.au-syd.bluemix.net", "https://api.eu-gb.bluemix.net"}

const GREEN = "\x1b[1;36m"
const RED = "\x1b[1;31m"
const NOCOLOR = "\x1b[0m"
const YELLOW = "\x1b[1;33m"
const CYAN = "\x1b[0;36m"

/*
*	This is the struct implementing the interface defined by the core CLI. It can
*	be found at  "github.com/cloudfoundry/cli/plugin/plugin.go"
*
 */
type BCSyncPlugin struct{}

type HttpResponse struct {
	requestType string
	status      string
	body        string
	err         error
}

/*
*	This function must be implemented by any plugin because it is part of the
*	plugin interface defined by the core CLI.
*
*	Run(....) is the entry point when the core CLI is invoking a command defined
*	by a plugin. The first parameter, plugin.CliConnection, is a struct that can
*	be used to invoke cli commands. The second paramter, args, is a slice of
*	strings. args[0] will be the name of the command, and will be followed by
*	any additional arguments a cli user typed in.
*
*	Any error handling should be handled with the plugin itself (this means printing
*	user facing errors). The CLI will exit 0 if the plugin exits 0 and will exit
*	1 should the plugin exits nonzero.
 */
func (c *BCSyncPlugin) Run(cliConnection plugin.CliConnection, args []string) {
	if args[0] == "sync-app-dbs" {
		var appname, password string
		var dbs []string
		var err error
		loggedIn, _ := cliConnection.IsLoggedIn()
		if !loggedIn {
			fmt.Println("\nPlease login first via '" + YELLOW + "cf login" + NOCOLOR + "'\n")
			return
		}
		password = bcs_prompts.GetPassword()
		for i := 1; i < len(args); i++ {
			switch args[i] {
			case "-a":
				appname = args[i+1]
			case "-d":
				dbs = strings.Split(args[i+1], ",")
			}
		}
		if appname == "" {
			appname, err = bcs_prompts.GetAppName(cliConnection)
			if err != nil {
				fmt.Println(err)
				return
			}
		}
		var httpClient = &http.Client{}
		cloudantAccounts, err := cam.GetCloudantAccounts(cliConnection, httpClient, ENDPOINTS, appname, password)
		if len(dbs) == 0 {
			dbs = bcs_prompts.GetDatabase(httpClient, cloudantAccounts[0])
		}
		if err != nil {
			fmt.Println(RED + err.Error() + NOCOLOR)
			return
		}
		createReplicatorDatabases(httpClient, cloudantAccounts)
		for i := 0; i < len(dbs); i++ {
			shareDatabases(dbs[i], httpClient, cloudantAccounts)
			createReplicationDocuments(dbs[i], httpClient, cloudantAccounts)
		}
		deleteCookies(httpClient, cloudantAccounts)
	}
}

/*
*	Sends all necessary requests to link all databases. These
*	requests should generate documents in the target's
*	_replicator database.
 */
func createReplicationDocuments(db string, httpClient *http.Client, cloudantAccounts []cam.CloudantAccount) {
	fmt.Println("\nCreating replication documents for " + db + "\n")
	responses := make(chan HttpResponse)
	for i := 0; i < len(cloudantAccounts); i++ {
		account := cloudantAccounts[i]
		url := "http://" + account.Username + ".cloudant.com/_replicator"
		for j := 0; j < len(cloudantAccounts); j++ {
			if i != j {
				go func(httpClient *http.Client, account cam.CloudantAccount, source cam.CloudantAccount, db string) {
					rep := make(map[string]interface{})
					rep["_id"] = source.Username + "-" + db
					rep["source"] = source.Url + "/" + db
					rep["target"] = account.Url + "/" + db
					rep["create-target"] = false
					rep["continuous"] = true
					bd, _ := json.MarshalIndent(rep, " ", "  ")
					body := string(bd)
					headers := map[string]string{"Content-Type": "application/json", "Cookie": account.Cookie}
					resp, err := bcs_utils.MakeRequest(httpClient, "POST", url, body, headers)
					defer resp.Body.Close()
					respBody, _ := ioutil.ReadAll(resp.Body)
					if resp.Status != "409 Conflict" && resp.Status != "201 Created" {
						responses <- HttpResponse{requestType: "POST", status: resp.Status, body: string(respBody), err: errors.New("Trouble creating " + rep["_id"].(string) + " for '" + account.Endpoint + "'")}
					} else {
						responses <- HttpResponse{requestType: "POST", status: resp.Status, body: string(respBody), err: err}
					}
				}(httpClient, account, cloudantAccounts[j], db)
			}
		}
	}
	var resp []HttpResponse
	//waiting on requests to return
	for {
		select {
		case r := <-responses:
			if r.err != nil {
				fmt.Println(r.err)
				fmt.Println("Request status: " + r.status)
			}
			resp = append(resp, r)
		case <-time.After(50 * time.Millisecond):
			continue
		}
		if len(cloudantAccounts)*(len(cloudantAccounts)-1) == len(resp) {
			break
		}
	}
	close(responses)
}

/*
*	Sends a request to create a _replicator database for each
*	Cloudant Account.
 */
func createReplicatorDatabases(httpClient *http.Client, cloudantAccounts []cam.CloudantAccount) {
	fmt.Println("\nCreating replicator databases\n")
	responses := make(chan HttpResponse)
	for i := 0; i < len(cloudantAccounts); i++ {
		go func(httpClient *http.Client, account cam.CloudantAccount) {
			url := "http://" + account.Username + ".cloudant.com/_replicator"
			headers := map[string]string{"Content-Type": "application/json", "Cookie": account.Cookie}
			resp, err := bcs_utils.MakeRequest(httpClient, "PUT", url, "", headers)
			defer resp.Body.Close()
			respBody, _ := ioutil.ReadAll(resp.Body)
			if resp.Status != "201 Created" && resp.Status != "412 Precondition Failed" {
				responses <- HttpResponse{requestType: "PUT", status: resp.Status, body: string(respBody), err: errors.New(account.Endpoint + " replicator database status unknown")}
			} else {
				responses <- HttpResponse{requestType: "PUT", status: resp.Status, body: string(respBody), err: err}
			}
		}(httpClient, cloudantAccounts[i])
	}
	var resp []HttpResponse
	//waiting on requests to return
	for {
		select {
		case r := <-responses:
			if r.err != nil {
				fmt.Println(r.err)
				fmt.Println("Request status: " + r.status)
			}
			resp = append(resp, r)
		case <-time.After(50 * time.Millisecond):
			continue
		}
		if len(cloudantAccounts) == len(resp) {
			break
		}
	}
	close(responses)
}

func getPermissions(db string, httpClient *http.Client, account cam.CloudantAccount) HttpResponse {
	url := "http://" + account.Username + ".cloudant.com/_api/v2/db/" + db + "/_security"
	headers := map[string]string{"Cookie": account.Cookie}
	resp, err := bcs_utils.MakeRequest(httpClient, "GET", url, "", headers)
	defer resp.Body.Close()
	respBody, _ := ioutil.ReadAll(resp.Body)
	return HttpResponse{requestType: "GET", status: resp.Status, body: string(respBody), err: err}
}

func modifyPermissions(perms string, db string, httpClient *http.Client, account cam.CloudantAccount, cloudantAccounts []cam.CloudantAccount) HttpResponse {
	var parsed map[string]interface{}
	json.Unmarshal([]byte(perms), &parsed)
	for i := 0; i < len(cloudantAccounts); i++ {
		if account.Username != cloudantAccounts[i].Username {
			temp_parsed := make(map[string]interface{})
			if parsed["cloudant"] != nil {
				temp_parsed = parsed["cloudant"].(map[string]interface{})
			}
			if temp_parsed[cloudantAccounts[i].Username] == nil {
				temp_parsed[cloudantAccounts[i].Username] = []string{"_reader", "_replicator"}
			} else {
				currPerms := temp_parsed[cloudantAccounts[i].Username].([]interface{})
				addRead := true
				addRep := true
				for j := 0; j < len(currPerms); j++ {
					if currPerms[j].(string) == "_reader" {
						addRead = false
					}
					if currPerms[j].(string) == "_replicator" {
						addRep = false
					}
				}
				if addRead {
					currPerms = append(currPerms, "_reader")
				}
				if addRep {
					currPerms = append(currPerms, "_replicator")
				}
				temp_parsed[cloudantAccounts[i].Username] = currPerms
			}
			parsed["cloudant"] = map[string]interface{}(temp_parsed)
		}
	}
	url := "http://" + account.Username + ".cloudant.com/_api/v2/db/" + db + "/_security"
	bd, _ := json.MarshalIndent(parsed, " ", "  ")
	body := string(bd)
	headers := map[string]string{"Content-Type": "application/json", "Cookie": account.Cookie}
	resp, err := bcs_utils.MakeRequest(httpClient, "PUT", url, body, headers)
	defer resp.Body.Close()
	respBody, _ := ioutil.ReadAll(resp.Body)
	return HttpResponse{requestType: "PUT", status: resp.Status, body: string(respBody), err: err}
}

/*
*	Retrieves the current permissions for each database that is to be
*	replicated and modifies those permissions to allow read and replicate
*	permissions for every other database
 */
func shareDatabases(db string, httpClient *http.Client, cloudantAccounts []cam.CloudantAccount) {
	fmt.Println("\nModifying database permissions for " + db + "\n")
	responses := make(chan HttpResponse)
	for i := 0; i < len(cloudantAccounts); i++ {
		go func(db string, httpClient *http.Client, account cam.CloudantAccount, cloudantAccounts []cam.CloudantAccount) {
			r := getPermissions(db, httpClient, account)
			if r.status == "200 OK" && r.err == nil {
				responses <- r
				responses <- modifyPermissions(r.body, db, httpClient, account, cloudantAccounts)
			} else {
				r.err = errors.New("Permissions GET request failed for '" + account.Endpoint + "'")
				responses <- r
				responses <- HttpResponse{requestType: "PUT", status: "", body: "", err: errors.New("Did not execute for '" + account.Endpoint + "' due to GET failure")}
			}
		}(db, httpClient, cloudantAccounts[i], cloudantAccounts)
	}
	var resp []HttpResponse
	//waiting on requests to return
	for {
		select {
		case r := <-responses:
			fmt.Println(r.requestType, r.status)
			if r.err != nil {
				fmt.Println("with an error", r.err)
			}
			resp = append(resp, r)
		case <-time.After(50 * time.Millisecond):
			continue
		}
		if len(cloudantAccounts)*2 == len(resp) {
			break
		}
	}
	close(responses)
}

/*
*	Deletes the cookies that were used to authenticate the api calls
 */
func deleteCookies(httpClient *http.Client, cloudantAccounts []cam.CloudantAccount) {
	fmt.Println("\nDeleting Cookies\n")
	for i := 0; i < len(cloudantAccounts); i++ {
		account := cloudantAccounts[i]
		url := "http://" + account.Username + ".cloudant.com/_session"
		body := "name=" + account.Username + "&password=" + account.Password
		headers := map[string]string{"Content-Type": "application/x-www-form-urlencoded", "Cookie": account.Cookie}
		resp, _ := bcs_utils.MakeRequest(httpClient, "POST", url, body, headers)
		resp.Body.Close()
	}
}

/*
* 	For debugging purposes
 */
func printResponse(resp *http.Response) {
	fmt.Println("Status: " + resp.Status)
	fmt.Println("Header: ", resp.Header)
	body, _ := ioutil.ReadAll(resp.Body)
	fmt.Println("Body: ", string(body))
}

/*
*	This function must be implemented as part of the	plugin interface
*	defined by the core CLI.
*
*	GetMetadata() returns a PluginMetadata struct. The first field, Name,
*	determines the name of the plugin which should generally be without spaces.
*	If there are spaces in the name a user will need to properly quote the name
*	during uninstall otherwise the name will be treated as seperate arguments.
*	The second value is a slice of Command structs. Our slice only contains one
*	Command Struct, but could contain any number of them. The first field Name
*	defines the command `cf basic-plugin-command` once installed into the CLI. The
*	second field, HelpText, is used by the core CLI to display help information
*	to the user in the core commands `cf help`, `cf`, or `cf -h`.
 */
func (c *BCSyncPlugin) GetMetadata() plugin.PluginMetadata {
	return plugin.PluginMetadata{
		Name: "bluemix-cloudant-sync",
		Version: plugin.VersionType{
			Major: 1,
			Minor: 0,
			Build: 0,
		},
		MinCliVersion: plugin.VersionType{
			Major: 6,
			Minor: 7,
			Build: 0,
		},
		Commands: []plugin.Command{
			plugin.Command{
				Name:     "sync-app-dbs",
				HelpText: "synchronizes Cloudant databases for multi-regional apps",

				// UsageDetails is optional
				// It is used to show help of usage of each command
				UsageDetails: plugin.Usage{
					Usage: "cf sync-app-dbs [-a APP] [-d DATABASE] [-p PASSWORD]\n",
					Options: map[string]string{
						"-a": "App",
						"-d": "Database",
						"-p": "Password"},
				},
			},
		},
	}
}

/*
* Unlike most Go programs, the `Main()` function will not be used to run all of the
* commands provided in your plugin. Main will be used to initialize the plugin
* process, as well as any dependencies you might require for your
* plugin.
 */
func main() {
	// Any initialization for your plugin can be handled here
	//
	// Note: to run the plugin.Start method, we pass in a pointer to the struct
	// implementing the interface defined at "github.com/cloudfoundry/cli/plugin/plugin.go"
	//
	// Note: The plugin's main() method is invoked at install time to collect
	// metadata. The plugin will exit 0 and the Run([]string) method will not be
	// invoked.
	plugin.Start(new(BCSyncPlugin))
	// Plugin code should be written in the Run([]string) method,
	// ensuring the plugin environment is bootstrapped.
}
