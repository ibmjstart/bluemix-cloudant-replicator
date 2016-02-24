package main

import (
	"bufio"
	"fmt"
	"github.com/cloudfoundry/cli/plugin"
	//"github.com/cloudfoundry/cli/plugin/models"
	"bytes"
	"io/ioutil"
	"net/http"
	"os"
	//"reflect"
	"regexp"
	"strings"
)

var ENDPOINTS = [3]string{"https://api.ng.bluemix.net", "https://api.au-syd.bluemix.net", "https://api.eu-gb.bluemix.net"}

/*
*	This is the struct implementing the interface defined by the core CLI. It can
*	be found at  "github.com/cloudfoundry/cli/plugin/plugin.go"
*
 */
type BCSyncPlugin struct{}

type CloudantCreds struct {
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
	var appName string
	if len(args) > 1 {
		appName = args[1]
	} else {
		appName = getAppName(cliConnection)
	}
	var httpClient = &http.Client{}
	cloudantAccounts, err := getCloudantAccounts(cliConnection, httpClient, appName)
	if err != nil {
		return
	}
	deleteCookies(httpClient, cloudantAccounts)
}

/*
func initialPrompt() (string, string){
	reader := bufio.NewReader(os.Stdin)
	fmt.Println("\nWhich app's databases would you like to sync?")
	appName, _ := reader.ReadString('\n')
	appName = strings.TrimRight(appName, "\n")

}
*/

func getAppName(cliConnection plugin.CliConnection) string {
	reader := bufio.NewReader(os.Stdin)
	apps_list, _ := cliConnection.GetApps()
	fmt.Println("\nCurrent apps:\n")
	for i := 0; i < len(apps_list); i++ {
		fmt.Println(apps_list[i].Name)
	}
	fmt.Println("\nWhich app's databases would you like to sync?")
	appName, _ := reader.ReadString('\n')
	appName = strings.TrimRight(appName, "\n")
	fmt.Println("\n")
	return appName
}

func deleteCookies(httpClient *http.Client, cloudantAccounts [3]CloudantCreds) {
	for i := 0; i < len(cloudantAccounts); i++ {
		cred := cloudantAccounts[i]
		url := "http://" + cred.username + ".cloudant.com/_session"
		body := "name=" + cred.username + "&password=" + cred.password
		req, err := http.NewRequest("POST", url, bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Cookie", cred.cookie)
		resp, err := httpClient.Do(req)
		if err != nil {
			fmt.Println(err)
		}
		//Just for debugging purposes
		fmt.Println("response Status:", resp.Status)
		fmt.Println("response Headers:", resp.Header)
		respBody, _ := ioutil.ReadAll(resp.Body)
		fmt.Println("response Body:", string(respBody))
		resp.Body.Close()
	}
}

func getCloudantAccounts(cliConnection plugin.CliConnection, httpClient *http.Client, appName string) ([3]CloudantCreds, error) {
	var cloudantAccounts [3]CloudantCreds
	for i := 0; i < len(ENDPOINTS); i++ {
		cliConnection.CliCommand("api", ENDPOINTS[i])
		cliConnection.CliCommand("login")
		cred, err := getCreds(cliConnection, appName)
		if err != nil {
			fmt.Println(err)
			fmt.Println("Make sure that you are giving is a valid app IN ALL REGIONS and try again")
			return cloudantAccounts, err
		}
		cred.cookie = getCookie(cred, httpClient)
		cloudantAccounts[i] = cred
	}
	return cloudantAccounts, nil
}

func getCreds(cliConnection plugin.CliConnection, appName string) (CloudantCreds, error) {
	var creds CloudantCreds
	env, err := cliConnection.CliCommandWithoutTerminalOutput("env", appName)
	if err != nil {
		return creds, err
	}
	for i := 0; i < len(env); i++ {
		if strings.Index(env[i], "cloudantNoSQLDB") != -1 {
			user_reg, _ := regexp.Compile("\"username\": \"([\x00-\x7F]+)\"")
			pass_reg, _ := regexp.Compile("\"password\": \"([\x00-\x7F]+)\"")
			url_reg, _ := regexp.Compile("\"url\": \"([\x00-\x7F]+)\"")
			creds.username = strings.Split(user_reg.FindString(env[i]), "\"")[3]
			creds.password = strings.Split(pass_reg.FindString(env[i]), "\"")[3]
			creds.url = strings.Split(url_reg.FindString(env[i]), "\"")[3]
			break
		}
	}
	return creds, nil
}

func getCookie(cred CloudantCreds, httpClient *http.Client) string {
	url := "http://" + cred.username + ".cloudant.com/_session"
	reqBody := "name=" + cred.username + "&password=" + cred.password
	fmt.Println(reqBody)
	req, err := http.NewRequest("POST", url, bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := httpClient.Do(req)
	if err != nil {
		fmt.Println(err)
	}
	//Just for debugging purposes
	fmt.Println("response Status:", resp.Status)
	fmt.Println("response Headers:", resp.Header)
	respBody, _ := ioutil.ReadAll(resp.Body)
	fmt.Println("response Body:", string(respBody))
	resp.Body.Close()
	cookie := resp.Header.Get("Set-Cookie")
	return cookie
}

/*
//Did not need to look for the service in this manner since the service credentials are with the app and not the service itself
//plus, the app only had user definied environment variables associated with it in the GetAppModel.
func getCloudantServices(cliConnection plugin.CliConnection, app plugin_models.GetAppModel) plugin_models.GetService_Model {
	var cloudantService plugin_models.GetService_Model
	services := app.Services
	for i := 0; i < len(services); i++ {
		s, _ := cliConnection.GetService(services[i].Name)
		if s.ServiceOffering.Name == "cloudantNoSQLDB" {
			fmt.Println(s.Name)
			fmt.Println(reflect.TypeOf(s))
			cloudantService = s
			break
		}
	}
	return cloudantService
}
*/

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
					Usage: "sync-app-dbs\n   cf sync-app-dbs",
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
