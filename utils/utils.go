package bcs_utils

import (
	"bytes"
	"fmt"
	"github.com/cloudfoundry/cli/cf/terminal"
	"log"
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
	var resp []HttpResponse
	for {
		select {
		case r := <-responses:
			if r.Err != nil {
				CheckErrorNonFatal(r.Err)
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

func CheckErrorNonFatal(err error) {
	if err != nil {
		fmt.Println(terminal.ColorizeBold("FAILED", 31))
		fmt.Println(err.Error())
	}
}

func CheckErrorFatal(err error) {
	if err != nil {
		fmt.Println(terminal.ColorizeBold("FAILED", 31))
		fmt.Println(err.Error())
		log.Fatal(err.Error())
	}

}
