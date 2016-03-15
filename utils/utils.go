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

/*
*	Requests all databases for a given Cloudant account
*	and returns them as a string array
 */
func GetAllDatabases(httpClient *http.Client, account cam.CloudantAccount) []string {
	url := "https://" + account.Username + ".cloudant.com/_all_dbs"
	headers := map[string]string{"Cookie": account.Cookie}
	resp, _ := MakeRequest(httpClient, "GET", url, "", headers)
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

func HandleFlags(args []string) (string, []string, string, bool) {
	var appname, password string
	var dbs []string
	all_dbs := false
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
		}
	}
	return appname, dbs, password, all_dbs
}
