package bcr_prompts

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	cf_terminal "github.com/cloudfoundry/cli/cf/terminal"
	"github.com/cloudfoundry/cli/plugin"
	"github.com/ibmjstart/bluemix-cloudant-replicator/CloudantAccountModel"
	"github.com/ibmjstart/bluemix-cloudant-replicator/utils"
	"golang.org/x/crypto/ssh/terminal"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
)

func init() {
	cf_terminal.InitColorSupport()
}

func GetPassword() string {
	fmt.Print("\nBluemix password to log in across multiple regions.\n")
	fmt.Print("\nPassword" + cf_terminal.ColorizeBold(">", 36))
	pw, _ := terminal.ReadPassword(0)
	fmt.Println("\n")
	return string(pw)
}

/*
*	Requests all databases for a given Cloudant account
*	and returns them as a string array
 */
func GetAllDatabases(httpClient *http.Client, account cam.CloudantAccount) []string {
	url := "http://" + account.Username + ".cloudant.com/_all_dbs"
	headers := map[string]string{"Cookie": account.Cookie}
	resp, _ := bcr_utils.MakeRequest(httpClient, "GET", url, "", headers)
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
*	Lists all databases for a specified CloudantAccount and
*	prompts the user to select one
 */
func GetDatabases(httpClient *http.Client, account cam.CloudantAccount) ([]string, error) {
	reader := bufio.NewReader(os.Stdin)
	all_dbs := GetAllDatabases(httpClient, account)
	if len(all_dbs) == 0 {
		return all_dbs, errors.New("No databases found for CloudantNoSQLDB service in '" +
			cf_terminal.ColorizeBold(account.Endpoint, 36) + "'")
	}
	fmt.Println("Current databases:\n")
	for i := 0; i < len(all_dbs); i++ {
		fmt.Println(strconv.Itoa(i+1) + ". " + cf_terminal.ColorizeBold(all_dbs[i], 36))
	}
	if len(all_dbs) > 1 {
		fmt.Println(strconv.Itoa(len(all_dbs)+1) + ". sync all databases")
	}
	fmt.Print("\nWhich database would you like to sync?" + cf_terminal.ColorizeBold(">", 36))
	d, _, _ := reader.ReadLine()
	selected_dbs := strings.Split(string(d), ",")
	fmt.Println()
	var dbs []string
	for i := 0; i < len(selected_dbs); i++ {
		if j, err := strconv.Atoi(selected_dbs[i]); err == nil {
			if j <= len(all_dbs) && j > 0 {
				dbs = append(dbs, all_dbs[j-1])
			} else if j == len(all_dbs)+1 && len(all_dbs) > 1 {
				return all_dbs, nil
			} else {
				return all_dbs, errors.New("Index out of range")
			}
		} else {
			if bcr_utils.IsValid(selected_dbs[i], all_dbs) {
				dbs = append(dbs, selected_dbs[i])
			} else {
				return all_dbs, errors.New(selected_dbs[i] + " is not a valid database")
			}
		}
	}
	return dbs, nil
}

/*
*	Lists all current apps and prompts user to select one
 */
func GetAppName(cliConnection plugin.CliConnection) (string, error) {
	reader := bufio.NewReader(os.Stdin)
	currEndpoint, _ := cliConnection.ApiEndpoint()
	currOrg, err := cliConnection.GetCurrentOrg()
	apps_list, _ := bcr_utils.GetAllApps(cliConnection)
	if err != nil || currOrg.Name == "" {
		return "", errors.New("Difficulty pinpointing current org. Please log in again and point to the desired org.")
	}
	if len(apps_list) > 0 {
		fmt.Println("\nAll existing apps in org '" + cf_terminal.ColorizeBold(currOrg.Name, 36) + "' at '" + cf_terminal.ColorizeBold(currEndpoint, 36) + "':\n")
		for i := 0; i < len(apps_list); i++ {
			fmt.Println(strconv.Itoa(i+1) + ". " + cf_terminal.ColorizeBold(apps_list[i], 36))
		}
	} else {
		return "", errors.New("No apps found in org '" + cf_terminal.ColorizeBold(currOrg.Name, 36) + "' at '" +
			cf_terminal.ColorizeBold(currEndpoint, 36) + "'.\nPlease log in and point to an org with available apps.\n")
	}
	fmt.Print("\nFrom the list above, which app's databases would you like to sync?" + cf_terminal.ColorizeBold(">", 36))
	appName, _, _ := reader.ReadLine()
	fmt.Println()
	if i, err := strconv.Atoi(string(appName)); err == nil {
		if i <= len(apps_list) && i > 0 {
			return apps_list[i-1], nil
		} else {
			return "", errors.New("Index out of range")
		}
	}
	if !bcr_utils.IsValid(string(appName), apps_list) {
		return "", errors.New(string(appName) + " is not a valid app")
	}
	return string(appName), nil
}
