package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/cloudfoundry/cli/plugin"
	"io/ioutil"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
)

var ENDPOINTS = []string{"https://api.ng.bluemix.net", "https://api.au-syd.bluemix.net", "https://api.eu-gb.bluemix.net"}

/*
*	This is the struct implementing the interface defined by the core CLI. It can
*	be found at  "github.com/cloudfoundry/cli/plugin/plugin.go"
*
 */
type BCSyncPlugin struct{}

type CloudantAccount struct {
	username string
	password string
	url      string
	cookie   string
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
		var appName string
		var db string
		var password string
		currOrg, _ := cliConnection.GetCurrentOrg()
		org := currOrg.Name
		loggedIn, _ := cliConnection.IsLoggedIn()
		if !loggedIn {
			fmt.Println("\nPlease login first via 'cf login'\n")
			return
		}
		for i := 1; i < len(args); i++ {
			switch args[i] {
			case "-a":
				appName = args[i+1]
			case "-d":
				db = args[i+1]
			case "-p":
				password = args[i+1]
			}
		}
		if appName == "" {
			appName = getAppName(cliConnection)
		}
		var httpClient = &http.Client{}
		cloudantAccounts, err := getCloudantAccounts(cliConnection, httpClient, appName, password, org)
		if err != nil {
			return
		}
		if db == "" {
			db = getDatabase(httpClient, cloudantAccounts[0])
		}
		err = shareDatabases(db, httpClient, cloudantAccounts)
		if err != nil {
			fmt.Println(err)
			return
		}
		createReplicatorDatabases(httpClient, cloudantAccounts)
		createReplicationDocuments(db, httpClient, cloudantAccounts)
		deleteCookies(httpClient, cloudantAccounts)
	}
}

/*
*	Sends all necessary requests to link all databases. These
*	requests should generate documents in the target's
*	_replicator database.
 */
func createReplicationDocuments(db string, httpClient *http.Client, cloudantAccounts []CloudantAccount) {
	fmt.Println("\nCreating replication documents\n")
	for i := 0; i < len(cloudantAccounts); i++ {
		account := cloudantAccounts[i]
		url := "http://" + account.username + ".cloudant.com/_replicator"
		for j := 0; j < len(cloudantAccounts); j++ {
			if i != j {
				rep := make(map[string]interface{})
				rep["source"] = cloudantAccounts[j].url + "/" + db
				rep["target"] = account.url + "/" + db
				rep["create-target"] = false
				rep["continuous"] = true
				bd, _ := json.MarshalIndent(rep, " ", "  ")
				body := string(bd)
				headers := map[string]string{"Content-Type": "application/json", "Cookie": account.cookie}
				resp, _ := makeRequest(httpClient, "POST", url, body, headers)
				//printResponse(resp)
				resp.Body.Close()
			}
		}
	}
}

/*
*	Sends a request to create a _replicator database for each
*	Cloudant Account.
 */
func createReplicatorDatabases(httpClient *http.Client, cloudantAccounts []CloudantAccount) {
	fmt.Println("\nCreating replicator databases\n")
	for i := 0; i < len(cloudantAccounts); i++ {
		account := cloudantAccounts[i]
		url := "http://" + account.username + ".cloudant.com/_replicator"
		headers := map[string]string{"Content-Type": "application/json", "Cookie": account.cookie}
		resp, _ := makeRequest(httpClient, "PUT", url, "", headers)
		//printResponse(resp)
		resp.Body.Close()
	}
}

/*
*	Retrieves the current permissions for each database that is to be
*	replicated and modifies those permissions to allow read and replicate
*	permissions for every other database
 */
func shareDatabases(db string, httpClient *http.Client, cloudantAccounts []CloudantAccount) error {
	fmt.Println("\nModifying database permissions\n")
	for i := 0; i < len(cloudantAccounts); i++ {
		account := cloudantAccounts[i]
		url := "http://" + account.username + ".cloudant.com/_api/v2/db/" + db + "/_security"
		headers := map[string]string{"Cookie": account.cookie}
		resp, _ := makeRequest(httpClient, "GET", url, "", headers)
		if resp.Status != "200 OK" {
			return errors.New("Makes sure that a valid database is being given")
		}
		respBody, _ := ioutil.ReadAll(resp.Body)
		perms := string(respBody)
		resp.Body.Close()
		var parsed map[string]interface{}
		json.Unmarshal([]byte(perms), &parsed)
		for j := 0; j < len(cloudantAccounts); j++ {
			if i != j {
				temp_parsed := make(map[string]interface{})
				if parsed["cloudant"] != nil {
					temp_parsed = parsed["cloudant"].(map[string]interface{})
				}
				temp_parsed[cloudantAccounts[j].username] = []string{"_reader", "_replicator"}
				parsed["cloudant"] = map[string]interface{}(temp_parsed)
			}
		}
		bd, _ := json.MarshalIndent(parsed, " ", "  ")
		body := string(bd)
		headers = map[string]string{"Content-Type": "application/json", "Cookie": account.cookie}
		resp, _ = makeRequest(httpClient, "PUT", url, body, headers)
		//printResponse(resp)
		resp.Body.Close()
	}
	return nil
}

/*
*	Lists all databases for a specified CloudantAccount and
*	prompts the user to select one
 */
func getDatabase(httpClient *http.Client, account CloudantAccount) string {
	reader := bufio.NewReader(os.Stdin)
	dbs := getAllDatabases(httpClient, account)
	fmt.Println("Current databases:")
	for i := 0; i < len(dbs); i++ {
		fmt.Println(strconv.Itoa(i+1) + ". " + dbs[i])
	}
	fmt.Println("\nWhich database would you like to sync?")
	db, _, _ := reader.ReadLine()
	fmt.Println()
	if i, err := strconv.Atoi(string(db)); err == nil {
		return dbs[i-1]
	}
	return string(db)
}

/*
*	Requests all databases for a given Cloudant account
*	and returns them as a string array
 */
func getAllDatabases(httpClient *http.Client, account CloudantAccount) []string {
	url := "http://" + account.username + ".cloudant.com/_all_dbs"
	headers := map[string]string{"Cookie": account.cookie}
	resp, _ := makeRequest(httpClient, "GET", url, "", headers)
	respBody, _ := ioutil.ReadAll(resp.Body)
	dbsStr := string(respBody)
	var dbs []string
	json.Unmarshal([]byte(dbsStr), &dbs)
	resp.Body.Close()
	return dbs
}

/*
*	Lists all current apps and prompts user to select one
 */
func getAppName(cliConnection plugin.CliConnection) string {
	reader := bufio.NewReader(os.Stdin)
	apps_list, _ := cliConnection.GetApps()
	if len(apps_list) > 0 {
		fmt.Println("\nCurrent apps:\n")
		for i := 0; i < len(apps_list); i++ {
			fmt.Println(strconv.Itoa(i+1) + ". " + apps_list[i].Name)
		}
	}
	fmt.Println("\nWhich app's databases would you like to sync?")
	appName, _, _ := reader.ReadLine()
	fmt.Println()
	if i, err := strconv.Atoi(string(appName)); err == nil {
		return apps_list[i-1].Name
	}
	return string(appName)
}

/*
*	Deletes the cookies that were used to authenticate the api calls
 */
func deleteCookies(httpClient *http.Client, cloudantAccounts []CloudantAccount) {
	fmt.Println("\nDeleting Cookies\n")
	for i := 0; i < len(cloudantAccounts); i++ {
		account := cloudantAccounts[i]
		url := "http://" + account.username + ".cloudant.com/_session"
		body := "name=" + account.username + "&password=" + account.password
		headers := map[string]string{"Content-Type": "application/x-www-form-urlencoded", "Cookie": account.cookie}
		resp, _ := makeRequest(httpClient, "POST", url, body, headers)
		//printResponse(resp)
		resp.Body.Close()
	}
}

/*
*	Cycles through all endpoints and retrieves the Cloudant
*	credentials for the specified app in each region.
 */
func getCloudantAccounts(cliConnection plugin.CliConnection, httpClient *http.Client, appName string, password string, org string) ([]CloudantAccount, error) {
	cloudantAccounts := make([]CloudantAccount, len(ENDPOINTS))
	username, _ := cliConnection.Username()
	startingEndpoint, _ := cliConnection.ApiEndpoint()
	skipFirst := startingEndpoint == ENDPOINTS[0]
	fmt.Println("\nRetrieving cookies for Cloudant authentication\n")
	for i := 0; i < len(ENDPOINTS); i++ {
		if !skipFirst || i != 0 {
			cliConnection.CliCommandWithoutTerminalOutput("api", ENDPOINTS[i])
			if password != "" {
				if org != "" {
					cliConnection.CliCommandWithoutTerminalOutput("login", "-u", username, "-p", password, "-o", org)
				} else {
					cliConnection.CliCommand("login", "-u", username, "-p", password)
				}
			} else {
				cliConnection.CliCommand("login", "-u", username, "-o", org)
			}
		}
		account, err := getAccount(cliConnection, appName)
		if err != nil {
			fmt.Println("Make sure that you are giving an app that exists IN ALL REGIONS and try again")
			return cloudantAccounts, err
		}
		account.cookie = getCookie(account, httpClient)
		cloudantAccounts[i] = account
	}
	cliConnection.CliCommandWithoutTerminalOutput("api", startingEndpoint) //point back to where the user started
	if password != "" {
		if org != "" {
			cliConnection.CliCommandWithoutTerminalOutput("login", "-u", username, "-p", password, "-o", org)
		} else {
			cliConnection.CliCommand("login", "-u", username, "-p", password)
		}
	} else {
		cliConnection.CliCommand("login", "-u", username, "-o", org)
	}
	return cloudantAccounts, nil
}

/*
*	Parses the environment variables for an app in order to get the
*	Cloudant username, password, and url
 */
func getAccount(cliConnection plugin.CliConnection, appName string) (CloudantAccount, error) {
	var account CloudantAccount
	env, err := cliConnection.CliCommandWithoutTerminalOutput("env", appName)
	if err != nil {
		return account, err
	}
	for i := 0; i < len(env); i++ {
		if strings.Index(env[i], "cloudantNoSQLDB") != -1 {
			user_reg, _ := regexp.Compile("\"username\": \"([\x00-\x7F]+)\"")
			pass_reg, _ := regexp.Compile("\"password\": \"([\x00-\x7F]+)\"")
			url_reg, _ := regexp.Compile("\"url\": \"([\x00-\x7F]+)\"")
			account.username = strings.Split(user_reg.FindString(env[i]), "\"")[3]
			account.password = strings.Split(pass_reg.FindString(env[i]), "\"")[3]
			account.url = strings.Split(url_reg.FindString(env[i]), "\"")[3]
			break
		}
	}
	return account, nil
}

/*
*	Gets cookie for a specified CloudantAccount. This cookie is
*	used to authenticate all necessary api calls.
 */
func getCookie(account CloudantAccount, httpClient *http.Client) string {
	url := "http://" + account.username + ".cloudant.com/_session"
	body := "name=" + account.username + "&password=" + account.password
	headers := map[string]string{"Content-Type": "application/x-www-form-urlencoded"}
	//req, _ := http.NewRequest("POST", url, bytes.NewBufferString(reqBody))
	//req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	//resp, _ := httpClient.Do(req)
	resp, _ := makeRequest(httpClient, "POST", url, body, headers)
	cookie := resp.Header.Get("Set-Cookie")
	//printResponse(resp)
	resp.Body.Close()
	return cookie
}

/*
* 	Creates a new http request based on the params and sends it, returning the response.
 */
func makeRequest(httpClient *http.Client, rType string, url string, body string, headers map[string]string) (*http.Response, error) {
	req, _ := http.NewRequest(rType, url, bytes.NewBufferString(body))
	for header, value := range headers {
		req.Header.Set(header, value)
	}
	return httpClient.Do(req)

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
