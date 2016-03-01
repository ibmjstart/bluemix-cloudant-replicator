package bcs_prompts

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/cloudfoundry/cli/plugin"
	"github.com/ibmjstart/bluemix-cloudant-sync/CloudantAccountModel"
	"golang.org/x/crypto/ssh/terminal"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
)

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

func GetPassword() string {
	fmt.Print("\nYour password is necessary in order for bluemix-cloudant-sync to login across multiple regions.\n")
	fmt.Print("\nPassword: ")
	pw, _ := terminal.ReadPassword(0)
	fmt.Println("\n")
	return string(pw)
}

/*
*	Lists all databases for a specified CloudantAccount and
*	prompts the user to select one
 */
func GetDatabase(httpClient *http.Client, account cam.CloudantAccount) []string {
	reader := bufio.NewReader(os.Stdin)
	dbs := getAllDatabases(httpClient, account)
	fmt.Println("Current databases:\n")
	for i := 0; i < len(dbs); i++ {
		fmt.Println(strconv.Itoa(i+1) + ". " + dbs[i])
	}
	fmt.Println(strconv.Itoa(len(dbs)+1) + ". sync all databases")
	fmt.Println("\nWhich database would you like to sync?")
	db, _, _ := reader.ReadLine()
	selected_dbs := strings.Split(string(db), ",")
	fmt.Println()
	var d []string
	for i := 0; i < len(selected_dbs); i++ {
		if j, err := strconv.Atoi(selected_dbs[i]); err == nil {
			if j <= len(dbs) {
				d = append(d, dbs[j-1])
			} else if j == len(dbs)+1 {
				return dbs
			}
		} else {
			d = append(d, selected_dbs[i])
		}
	}
	return d
}

/*
*	Requests all databases for a given Cloudant account
*	and returns them as a string array
 */
func getAllDatabases(httpClient *http.Client, account cam.CloudantAccount) []string {
	url := "http://" + account.Username + ".cloudant.com/_all_dbs"
	headers := map[string]string{"Cookie": account.Cookie}
	resp, _ := makeRequest(httpClient, "GET", url, "", headers)
	respBody, _ := ioutil.ReadAll(resp.Body)
	dbsStr := string(respBody)
	var dbs []string
	json.Unmarshal([]byte(dbsStr), &dbs)
	resp.Body.Close()
	var noRepDbs []string
	for i := 0; i < len(dbs); i++ {
		if dbs[i] != "_replicator" {
			noRepDbs = append(noRepDbs, dbs[i])
		}
	}
	return noRepDbs
}

/*
*	Lists all current apps and prompts user to select one
 */
func GetAppName(cliConnection plugin.CliConnection) string {
	reader := bufio.NewReader(os.Stdin)
	apps_list, _ := cliConnection.GetApps()
	currEndpoint, _ := cliConnection.ApiEndpoint()
	currOrg, _ := cliConnection.GetCurrentOrg()
	if len(apps_list) > 0 {
		fmt.Println("\nThese are all existing apps in the org '" + currOrg.Name + "' and at '" + currEndpoint + "':\n")
		for i := 0; i < len(apps_list); i++ {
			fmt.Println(strconv.Itoa(i+1) + ". " + apps_list[i].Name)
		}
	}
	fmt.Println("\nFrom the list above, which app's databases would you like to sync?")
	appName, _, _ := reader.ReadLine()
	fmt.Println()
	if i, err := strconv.Atoi(string(appName)); err == nil {
		return apps_list[i-1].Name
	}
	return string(appName)
}
