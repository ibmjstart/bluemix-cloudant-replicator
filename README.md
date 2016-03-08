# bluemix-cloudant-replicator
A [cf cli](https://github.com/cloudfoundry/cli) plugin for configuring continuous replication between Cloudant databases in multiple regions of [IBM Bluemix](http://bluemix.net)

## Installation
1. download the appropriate binary
	* Mac OS X: [64-bit](https://github.com/ibmjstart/bluemix-cloudant-replicator/releases/download/0.1.0/bc-replicator_0.1.0_osx.zip)
	* Windows:  [64-bit](https://github.com/ibmjstart/bluemix-cloudant-replicator/releases/download/0.1.0/bc-replicator_0.1.0_win64.zip)
	* Linux:    [64-bit](https://github.com/ibmjstart/bluemix-cloudant-replicator/releases/download/0.1.0/bc-replicator_0.1.0_linux.zip)
2. install the plugin via **cf install-plugin PATH_TO_PLUGIN_BINARY** 
	* if you get a permission error run: **chmod +x PATH_TO_PLUGIN_BINARY** on the binary
3. verify the plugin installed by looking for it with **cf plugins** 

> If you've already installed the plugin and are updating, run **cf uninstall-plugin bluemix-cloudant-replicator** before the install.

***


## Usage

```
cf cloudant-replicate [-a APP] [-d DATABASE] [-p PASSWORD]
```
The plugin will

1. Use `PASSWORD` to log into each of the different Bluemix regions (using the org and space names of the current target)
2. Retrieve the credentials from the Cloudant service instance bound to `APP` in each region
3. Set up continuous replication between the database names passed via `DATABASE`

If you call the command with no arguments, it will interactively prompt you to choose your app and databases from your current cf target. The interactive mode will guide you to your app in each region if necessary.

#### Options

1. The **-a** option allows you to pass in your app name directly
2. The **-d** option allows you to pass in a comma-delimited list of cloudant database names to replicate (not to be confused with the cloudant service instance names which it automatically infers from the application) 
3. The **-p** option allows you to pass in your Bluemix password directly


##Notes and Assumptions

#### Assumptions

1. The specified app exists in all regions
2. The same org and space name are used across regions (this is not a problem when using the interactive mode)
3. There is only one Cloudant service bound to the app (the first set of credentials will be used if not)
4. Each Cloudant service has a database by the same name as the original

#### Notes

There may be a case where you do not want to use all locations or you may want to add additional endpoints. To do this, you must fork the project and modify ENDPOINTS(found in bc-replicator.go). When you do this, it is up to you to recompile the code and re-install the plugin following the same instructions found above.  The only difference is you will now point install-plugin to the newly compiled binary path.

This plugin was developed to help automate 'Step 3. Configure Cloudant replication' in [this](http://www.ibm.com/developerworks/cloud/library/cl-multi-region-bluemix-apps-with-cloudant-and-dyn-trs/index.html#cmt_4) article.
