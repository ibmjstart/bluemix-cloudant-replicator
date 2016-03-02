package bcs_utils

import (
	"bytes"
	"net/http"
)

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
