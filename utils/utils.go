package bcr_utils

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/cloudfoundry/cli/cf/terminal"
	"github.com/cloudfoundry/cli/plugin"
	"github.com/ibmjstart/bluemix-cloudant-replicator/CloudantAccountModel"
	"io/ioutil"
	"net/http"
	"strings"
	"time"
)

type HttpResponse struct {
	RequestType string
	Status      string
	Body        string
	Err         error
}

func init() {
	terminal.InitColorSupport()
}

func GetCurrentTarget(cliConnection plugin.CliConnection) (string, string, string, string) {
	endpoint, _ := cliConnection.ApiEndpoint()
	username, _ := cliConnection.Username()
	currOrg, _ := cliConnection.GetCurrentOrg()
	org := currOrg.Name
	currSpace, _ := cliConnection.GetCurrentSpace()
	space := currSpace.Name
	return endpoint, username, org, space
}

/*
* 	Creates a new http request based on the params and sends it, returning the response.
 */
func MakeRequest(httpClient *http.Client, rType string, url string, body string, headers map[string]string) (*http.Response, error) {
	req, _ := http.NewRequest(rType, url, bytes.NewBufferString(body))
	for header, value := range headers {
		req.Header.Set(header, value)
	}
	return httpClient.Do(req)
}

func CheckHttpResponses(responses chan HttpResponse, numCalls int) {
	if numCalls < 1 {
		return
	}
	var resp []HttpResponse
	for {
		select {
		case r := <-responses:
			if CheckErrorNonFatal(r.Err) {
				fmt.Println(r.RequestType)
				fmt.Println(r.Status)
				fmt.Println(r.Body)
			}
			resp = append(resp, r)
		case <-time.After(50 * time.Millisecond):
			continue
		}
		if numCalls == len(resp) {
			break
		}
	}
}

func CheckErrorNonFatal(err error) bool {
	if err != nil {
		fmt.Println(terminal.ColorizeBold("\nFAILED", 31))
		fmt.Println(err.Error())
		return true
	}
	return false
}

func CheckErrorFatal(err error) {
	if err != nil {
		fmt.Println(terminal.ColorizeBold("\nFAILED", 31))
		fmt.Println(err.Error())
		panic(err.Error())
	}

}

func IsValid(el string, elements []string) bool {
	for i := 0; i < len(elements); i++ {
		if el == elements[i] {
			return true
		}
	}
	return false
}

func GetAllApps(cliConnection plugin.CliConnection) ([]string, error) {
	var apps_list []string
	apps, err := cliConnection.GetApps()
	if err != nil {
		return apps_list, err
	}
	for i := 0; i < len(apps); i++ {
		apps_list = append(apps_list, apps[i].Name)
	}
	return apps_list, nil
}

func GetDatabases(httpClient *http.Client, account cam.CloudantAccount) []string {
	var dbs []string
	url := "https://" + account.Username + ".cloudant.com/_all_dbs"
	headers := map[string]string{"Cookie": account.Cookie}
	resp, err := MakeRequest(httpClient, "GET", url, "", headers)
	if CheckErrorNonFatal(err) {
		return dbs
	}
	defer resp.Body.Close()
	respBody, _ := ioutil.ReadAll(resp.Body)
	dbsStr := string(respBody)
	json.Unmarshal([]byte(dbsStr), &dbs)
	resp.Body.Close()
	return dbs
}

/*
*	Requests all databases for a given Cloudant account
*	and returns them as a string array
 */
func GetAllDatabases(httpClient *http.Client, cloudantAccounts []cam.CloudantAccount) []string {
	var all_dbs []string
	db_ch := make(chan []string)
	for i := 0; i < len(cloudantAccounts); i++ {
		go func(httpClient *http.Client, account cam.CloudantAccount) {
			dbs := GetDatabases(httpClient, account)
			db_ch <- dbs
		}(httpClient, cloudantAccounts[i])
	}
	num_responses := 0
	for {
		select {
		case dbs := <-db_ch:
			if len(dbs) != 0 {
				for j := 0; j < len(dbs); j++ {
					if dbs[j] != "_replicator" && !IsValid(dbs[j], all_dbs) {
						all_dbs = append(all_dbs, dbs[j])
					}
				}
			}
			num_responses += 1
		case <-time.After(50 * time.Millisecond):
			continue
		}
		if num_responses == len(cloudantAccounts) {
			break
		}
	}
	return all_dbs
}

func HandleFlags(args []string) (string, []string, string, bool, bool) {
	var appname, password string
	var dbs []string
	all_dbs := false
	create := false
	err := errors.New("Problem with command invocation. For help look to '" +
		terminal.ColorizeBold("cf help cloudant-replicate", 33) + "'")
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "-a":
			if i+1 >= len(args) {
				CheckErrorFatal(err)
			}
			appname = args[i+1]
		case "-d":
			if i+1 >= len(args) {
				CheckErrorFatal(err)
			}
			dbs = strings.Split(args[i+1], ",")
		case "-p":
			if i+1 >= len(args) {
				CheckErrorFatal(err)
			}
			password = args[i+1]
		case "--all-dbs":
			all_dbs = true
		case "--create":
			create = true
		}
	}
	return appname, dbs, password, all_dbs, create
}
