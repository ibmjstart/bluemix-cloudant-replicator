# bluemix-cloudant-replicator
### a cf cli plugin for replicating between cloudant databases


##Download Binaries

###### Mac:     [64-bit](https://github.com/ibmjstart/bluemix-cloudant-sync/blob/master/binaries/darwin/amd64/bc-replicator?raw=true)   
###### Windows: [64-bit](https://github.com/ibmjstart/bluemix-cloudant-sync/blob/master/binaries/windows/amd64/bc-replicator.exe?raw=true)    
###### Linux:   [64-bit](https://github.com/ibmjstart/bluemix-cloudant-sync/blob/master/binaries/linux/amd64/bc-replicator?raw=true)


## Installation
1. download binary (See Download Section above)
2. Then install the plugin with **cf install-plugin PATH_TO_PLUGIN_BINARY** 
	* If you get a permission error run: **chmod +x PATH_TO_PLUGIN_BINARY** on the binary
3. Verify the plugin installed by looking for it with **cf plugins** 

> If you've already installed the plugin and are updating, run **cf uninstall-plugin bluemix-cloudant-replicator** before the install.

***


## Usage

```
cf cloudant-replicate [-a APP] [-d DATABASE] [-p PASSWORD]
```
If you call the command with no assumptions, it will interactively prompt you to choose your app, and databases. The interactive mode of calling the command is much more forgiving. It attempts to guide you to your app in each region if necessary.

#### Options

1. The **-a** option allows you to pass in your app name directly
2. The **-d** option allows you to pass in your database name(or comma separated list) directly
3. The **-p** option allows you to pass in your password directly


##Notes and Assumptions

#### Assumptions

1. The specified app exists in all regions
2. The same org and space are used across regions (this is not a problem when using the interactive mode)
3. There is only one Cloudant service bound to the app (the first set of credentials will be used if not)

#### Notes

There may be a case where you do not want to use all locations or you may want to add additional endpoints. To do this, you must fork the project and modify ENDPOINTS(found in bc-replicator.go). When you do this, it is up to you to recompile the code and re-install the plugin following the same instructions found above.  The only difference is you will now point install-plugin to the newly compiled binary path.


Database names containing commas may result in issues. 


This plugin was developed to help automate the process found in the article [here](https://g01acxwass069.ahe.pok.ibm.com/cms/developerworks/cloud/library/cl-multi-region-bluemix-apps-with-cloudant-and-dyn-trs/index.html)
