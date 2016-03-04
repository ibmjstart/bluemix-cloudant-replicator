package bcr_utils

import (
	"bytes"
	"fmt"
	"github.com/cloudfoundry/cli/cf/terminal"
	"github.com/cloudfoundry/cli/plugin"
	//"log"
	"net/http"
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
