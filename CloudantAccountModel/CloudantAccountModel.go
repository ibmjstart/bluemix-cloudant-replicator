package cam

import (
	"errors"
	"fmt"
	"github.com/cloudfoundry/cli/cf/terminal"
	"github.com/cloudfoundry/cli/plugin"
	"github.com/ibmjstart/bluemix-cloudant-sync/utils"
	"net/http"
	"regexp"
	"strings"
	"time"
)

type CloudantAccount struct {
	Endpoint string
	Username string
	Password string
	Url      string
	Cookie   string
}

type CreateAccountResponse struct {
	account CloudantAccount
	err     error
}

func init() {
	terminal.InitColorSupport()
}

func createAccount(cliConnection plugin.CliConnection, httpClient *http.Client, env []string, endpoint string) CreateAccountResponse {
	account, err := parseCreds(env)
	if err != nil {
		return CreateAccountResponse{account: account, err: err}
	}
	account.Endpoint = endpoint
	account.Cookie = getCookie(account, httpClient)
	return CreateAccountResponse{account: account, err: nil}
}

/*
*	Cycles through all endpoints and retrieves the Cloudant
*	credentials for the specified app in each region.
 */
func GetCloudantAccounts(cliConnection plugin.CliConnection, httpClient *http.Client, ENDPOINTS []string, appname string, password string) ([]CloudantAccount, error) {
	var cloudantAccounts []CloudantAccount
	username, _ := cliConnection.Username()
	currOrg, _ := cliConnection.GetCurrentOrg()
	org := currOrg.Name
	currSpace, _ := cliConnection.GetCurrentSpace()
	space := currSpace.Name
	startingEndpoint, _ := cliConnection.ApiEndpoint()
	ch := make(chan CreateAccountResponse)
	for i := 0; i < len(ENDPOINTS); i++ {
		env, err := getAppEnv(cliConnection, username, password, org, ENDPOINTS[i], appname, space)
		//TODO:Tell the user the existing apps and handle error
		bcs_utils.CheckErrorFatal(err)
		go func(cliConnection plugin.CliConnection, httpClient *http.Client, env []string, endpoint string) {
			ch <- createAccount(cliConnection, httpClient, env, endpoint)
		}(cliConnection, httpClient, env, ENDPOINTS[i])
	}
	for {
		select {
		case r := <-ch:
			if r.err != nil {
				fmt.Println("with an error", r.err)
			}
			cloudantAccounts = append(cloudantAccounts, r.account)
		case <-time.After(50 * time.Millisecond):
			continue
		}
		if len(cloudantAccounts) == len(ENDPOINTS) {
			break
		}
	}
	close(ch)
	//TODO:Relogin no matter how it fails
	cliConnection.CliCommandWithoutTerminalOutput("login", "-u", username, "-p", password, "-o", org, "-a", startingEndpoint, "-s", space) //point back to where the user started
	return cloudantAccounts, nil
}

func parseCreds(env []string) (CloudantAccount, error) {
	var account CloudantAccount
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
*	Returns the result of "cf env APP"
 */
func getAppEnv(cliConnection plugin.CliConnection, username string, password string, org string, endpoint string, appname string, space string) ([]string, error) {
	fmt.Println("Retrieving CloudantNoSQLDB credentials for '" + terminal.ColorizeBold(appname, 36) + "' in '" + terminal.ColorizeBold(endpoint, 36) + "'\n")
	_, err := cliConnection.CliCommandWithoutTerminalOutput("login", "-u", username, "-p", password, "-o", org, "-a", endpoint, "-s", space)
	if err != nil {
		fmt.Println("Unable to log in to org " + terminal.ColorizeBold(org, 36) + " and/or space " + terminal.ColorizeBold(space, 36))
	}
	_, err = cliConnection.CliCommand("login", "-u", username, "-p", password, "-a", endpoint)
	bcs_utils.CheckErrorFatal(err)
	return cliConnection.CliCommandWithoutTerminalOutput("env", appname)
}

/*
*	Gets cookie for a specified CloudantAccount. This cookie is
*	used to authenticate all necessary api calls.
 */
func getCookie(account CloudantAccount, httpClient *http.Client) string {
	url := "http://" + account.Username + ".cloudant.com/_session"
	body := "name=" + account.Username + "&password=" + account.Password
	headers := map[string]string{"Content-Type": "application/x-www-form-urlencoded"}
	resp, _ := bcs_utils.MakeRequest(httpClient, "POST", url, body, headers)
	cookie := resp.Header.Get("Set-Cookie")
	resp.Body.Close()
	return cookie
}
