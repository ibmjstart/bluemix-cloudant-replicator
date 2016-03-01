package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/cloudfoundry/cli/plugin"
	"github.com/ibmjstart/bluemix-cloudant-sync/CloudantAccountModel"
	"github.com/ibmjstart/bluemix-cloudant-sync/prompts"
	"io/ioutil"
	"net/http"
	"regexp"
	"strings"
)

var ENDPOINTS = []string{"https://api.ng.bluemix.net", "https://api.au-syd.bluemix.net", "https://api.eu-gb.bluemix.net"}

/*
*	This is the struct implementing the interface defined by the core CLI. It can
*	be found at  "github.com/cloudfoundry/cli/plugin/plugin.go"
*
 */
type BCSyncPlugin struct{}

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
		var appName, password string
		var dbs []string
		loggedIn, _ := cliConnection.IsLoggedIn()
		if !loggedIn {
			fmt.Println("\nPlease login first via 'cf login'\n")
			return
		}
		password = bcs_prompts.GetPassword()
		for i := 1; i < len(args); i++ {
			switch args[i] {
			case "-a":
				appName = args[i+1]
			case "-d":
				dbs = strings.Split(args[i+1], ",")
			}
		}
		if appName == "" {
			appName = bcs_prompts.GetAppName(cliConnection)
		}
		var httpClient = &http.Client{}
		cloudantAccounts, err := getCloudantAccounts(cliConnection, httpClient, appName, password)
		if err != nil {
			fmt.Println(err)
			return
		}
		if len(dbs) == 0 {
			dbs = bcs_prompts.GetDatabase(httpClient, cloudantAccounts[0])
		}
		err = shareDatabases(dbs[0], httpClient, cloudantAccounts)
		if err != nil {
			fmt.Println(err)
			return
		}
		createReplicatorDatabases(httpClient, cloudantAccounts)
		createReplicationDocuments(dbs[0], httpClient, cloudantAccounts)
		deleteCookies(httpClient, cloudantAccounts)
	}
}

/*
*	Sends all necessary requests to link all databases. These
*	requests should generate documents in the target's
*	_replicator database.
 */
func createReplicationDocuments(db string, httpClient *http.Client, cloudantAccounts []cam.CloudantAccount) {
	fmt.Println("\nCreating replication documents\n")
	for i := 0; i < len(cloudantAccounts); i++ {
		account := cloudantAccounts[i]
		url := "http://" + account.Username + ".cloudant.com/_replicator"
		for j := 0; j < len(cloudantAccounts); j++ {
			if i != j {
				rep := make(map[string]interface{})
				rep["source"] = cloudantAccounts[j].Url + "/" + db
				rep["target"] = account.Url + "/" + db
				rep["create-target"] = false
				rep["continuous"] = true
				bd, _ := json.MarshalIndent(rep, " ", "  ")
				body := string(bd)
				headers := map[string]string{"Content-Type": "application/json", "Cookie": account.Cookie}
				resp, _ := makeRequest(httpClient, "POST", url, body, headers)
				resp.Body.Close()
			}
		}
	}
}

/*
*	Sends a request to create a _replicator database for each
*	Cloudant Account.
 */
func createReplicatorDatabases(httpClient *http.Client, cloudantAccounts []cam.CloudantAccount) {
	fmt.Println("\nCreating replicator databases\n")
	for i := 0; i < len(cloudantAccounts); i++ {
		account := cloudantAccounts[i]
		url := "http://" + account.Username + ".cloudant.com/_replicator"
		headers := map[string]string{"Content-Type": "application/json", "Cookie": account.Cookie}
		resp, _ := makeRequest(httpClient, "PUT", url, "", headers)
		resp.Body.Close()
	}
}

/*
*	Retrieves the current permissions for each database that is to be
*	replicated and modifies those permissions to allow read and replicate
*	permissions for every other database
 */
func shareDatabases(db string, httpClient *http.Client, cloudantAccounts []cam.CloudantAccount) error {
	fmt.Println("\nModifying database permissions\n")
	for i := 0; i < len(cloudantAccounts); i++ {
		account := cloudantAccounts[i]
		url := "http://" + account.Username + ".cloudant.com/_api/v2/db/" + db + "/_security"
		headers := map[string]string{"Cookie": account.Cookie}
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
				//TODO:CHECK FOR USER BEFORE CHANGING PERMISSIONS
				temp_parsed[cloudantAccounts[j].Username] = []string{"_reader", "_replicator"}
				parsed["cloudant"] = map[string]interface{}(temp_parsed)
			}
		}
		bd, _ := json.MarshalIndent(parsed, " ", "  ")
		body := string(bd)
		headers = map[string]string{"Content-Type": "application/json", "Cookie": account.Cookie}
		resp, _ = makeRequest(httpClient, "PUT", url, body, headers)
		resp.Body.Close()
	}
	return nil
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
		resp, _ := makeRequest(httpClient, "POST", url, body, headers)
		resp.Body.Close()
	}
}

/*
*	Cycles through all endpoints and retrieves the Cloudant
*	credentials for the specified app in each region.
 */
func getCloudantAccounts(cliConnection plugin.CliConnection, httpClient *http.Client, appName string, password string) ([]cam.CloudantAccount, error) {
	cloudantAccounts := make([]cam.CloudantAccount, len(ENDPOINTS))
	username, _ := cliConnection.Username()
	currOrg, _ := cliConnection.GetCurrentOrg()
	org := currOrg.Name
	startingEndpoint, _ := cliConnection.ApiEndpoint()
	fmt.Println("\nRetrieving cookies for Cloudant authentication\n")
	for i := 0; i < len(ENDPOINTS); i++ {
		cliConnection.CliCommandWithoutTerminalOutput("api", ENDPOINTS[i])
		cliConnection.CliCommandWithoutTerminalOutput("login", "-u", username, "-p", password, "-o", org)
		account, err := getAccount(cliConnection, appName)
		if err != nil {
			return cloudantAccounts, err
		}
		account.Cookie = getCookie(account, httpClient)
		cloudantAccounts[i] = account
	}
	fmt.Println("\nReturning you to your original api endpoint.\nTo avoid all of these manual logins, consider using the -p option.\n")
	cliConnection.CliCommandWithoutTerminalOutput("api", startingEndpoint) //point back to where the user started
	cliConnection.CliCommandWithoutTerminalOutput("login", "-u", username, "-p", password, "-o", org)
	return cloudantAccounts, nil
}

/*
*	Parses the environment variables for an app in order to get the
*	Cloudant username, password, and url
 */
func getAccount(cliConnection plugin.CliConnection, appName string) (cam.CloudantAccount, error) {
	var account cam.CloudantAccount
	env, err := cliConnection.CliCommandWithoutTerminalOutput("env", appName)
	if err != nil {
		return account, errors.New("Make sure that you are giving an app that exists IN ALL REGIONS and try again")
	}
	for i := 0; i < len(env); i++ {
		if strings.Index(env[i], "cloudantNoSQLDB") != -1 {
			user_reg, _ := regexp.Compile("\"username\": \"([\x00-\x7F]+)\"")
			pass_reg, _ := regexp.Compile("\"password\": \"([\x00-\x7F]+)\"")
			url_reg, _ := regexp.Compile("\"url\": \"([\x00-\x7F]+)\"")
			account.Username = strings.Split(user_reg.FindString(env[i]), "\"")[3]
			account.Password = strings.Split(pass_reg.FindString(env[i]), "\"")[3]
			account.Url = strings.Split(url_reg.FindString(env[i]), "\"")[3]
			break
		}
	}
	if account.Username == "" || account.Password == "" || account.Url == "" {
		return account, errors.New("\nProblem finding Cloudant credentials for app. Make sure that there is a valid 'cloudantNoSQLDB' service bound to your app.\n")
	}
	return account, nil
}

/*
*	Gets cookie for a specified CloudantAccount. This cookie is
*	used to authenticate all necessary api calls.
 */
func getCookie(account cam.CloudantAccount, httpClient *http.Client) string {
	url := "http://" + account.Username + ".cloudant.com/_session"
	body := "name=" + account.Username + "&password=" + account.Password
	headers := map[string]string{"Content-Type": "application/x-www-form-urlencoded"}
	resp, _ := makeRequest(httpClient, "POST", url, body, headers)
	cookie := resp.Header.Get("Set-Cookie")
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
