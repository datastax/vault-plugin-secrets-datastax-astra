# DataStax Astra DB Plugin for HashiCorp Vault

Welcome developers, security and database administrators, site reliability engineers, and operators. 

This open-source project, DataStax Astra DB Plugin for HashiCorp Vault, adds robust **token lifecycle management** features for Astra DB. Due to the nature of the Astra DB object hierarchy, by default, API tokens are not associated with specific users and currently the tokens do not have metadata descriptions. 

Without the plugin, it's easy to lose track of:

* Who created tokens
* The purpose of each token
* Which tokens are being used actively 

Consequently, there's no audit trail of who has downloaded and used tokens, and there's no tracking regarding who may have manually shared tokens with others. 

Astra DB Plugin for HashiCorp Vault solves these security management issues. To ensure that your token ownership and usage are well understood, the plugin gives you the ability to associate metadata with tokens -- such as the user who created each token, and what it is being used for -- and logs who has accessed the tokens. 

## What is HashiCorp Vault?

HashiCorp Vault is a widely-used solution across the tech industry. It's an identity-based secrets and encryption management system. HashiCorp Vault provides key-value encryption services that are gated by authentication and authorization methods. Access to tokens, secrets, and other sensitive data are securely stored, managed, and tightly controlled. Audit trails are provided. HashiCorp Vault is also extensible via a variety of interfaces, allowing plugins (including Astra DB Plugin for HashiCorp Vault) to contribute to this ecosystem.

Astra DB Plugin for HashiCorp Vault is offered as a Public Beta under the [Apache 2.0](../LICENSE.txt) license.

## Benefits

You can use Astra DB Plugin for HashiCorp Vault to: 	

* Log access to Astra DB tokens
* Create and revoke Astra DB tokens
* Associate metadata with Astra DB tokens for tracking purposes, in effect annotating each token's ownership &amp; purpose

The plugin's roadmap includes dynamic tokens; that is, the additional ability to rotate tokens based on a token's lifetime lease.

For related details, see the [HashiCorp Vault](https://www.hashicorp.com/products/vault) documentation.

## Video introduction

Check out this introductory, YouTube-hosted video on the DataStax Developers channel:

[![Astra DB Plugin for HashiCorp Vault video](https://img.youtube.com/vi/_NUK6-omsyA/0.jpg)](https://www.youtube.com/watch?v=_NUK6-omsyA)

Running time: 4:16

## Prerequisites

### If you'll build or contribute to the plugin code

If you haven't already, clone this [GitHub repo](https://github.com/datastax/vault-plugin-secrets-datastax-astra). 

You will need:

* [Golang](https://go.dev/doc/install) v1.17.9+ installed.
* A fully functional configured HashiCorp Vault instance, including the ability to run the `vault` command.
* An Astra DB account with an administrator's role - see [Roles and Permissions](#roles-and-permissions).
* A *root token* for each Astra DB organization that HashiCorp Vault will manage; the steps are covered in this topic. 

### If you'll install and use the plugin binary

You will need:

* An Astra DB account with an admin role - see [Roles and Permissions](#roles-and-permissions).
* A *root token* for each Astra DB organization that HashiCorp Vault will manage; the steps are covered in this topic. 

## About root tokens

Astra DB Plugin for HashiCorp Vault will use the root token (per organization) to subsequently generate additional tokens. Sample `vault` commands are presented in this topic. 

For information first on how to generate tokens with Astra DB, see:

* [Managing tokens in Astra DB console](https://docs.datastax.com/en/astra/docs/manage/org/managing-org.html#_manage_application_tokens)
* Or, [managing tokens in DevOps API](https://docs.datastax.com/en/astra/docs/manage/devops/devops-tokens.html)

To create root tokens that are then authorized to create new tokens, your Astra DB account must have an admin role.

## Astra DB roles

Any of the following Astra DB roles can create root tokens:

* Organization Administrator (recommended)
* Database Administrator
* Service Account Administrator
* User Administrator

For more, see [user roles and permissions](https://docs.datastax.com/en/astra/docs/manage/org/user-permissions.html).

## Pricing

Astra DB Plugin for HashiCorp Vault is free. See the HashiCorp Platform Vault site for its [enterprise pricing](https://cloud.hashicorp.com/products/vault/pricing) details. 

## Build steps - optional

If you elect to build the plugin from Go modules in our GitHub repo, follow these steps. Otherwise, you can use the provided binary.

1. Build the plugin:

	```bash
	go build -o vault/plugins/vault-plugin-secrets-datastax-astra cmd/vault-plugin-secrets-datastax-astra/main.go
	```

2. Enable the plugin in your Vault instance:

	```bash
	vault secrets enable -path=astra vault-plugin-secrets-datastax-astra
	```

## Setup plugin from binary distribution

1. Create a plugins directory where HashiCorp Vault will find the plugin. Example: `./vault/plugins`.  **IMPORTANT:** do not specify a symlinked directory.

2. Download the latest release Astra DB Plugin for HashiCorp Vault package for your operating system. In GitHub, navigate to the following directory, and click the relevant tarball to download it: https://github.com/datastax/vault-plugin-secrets-datastax-astra/releases/tag/v0.1.0. 

3. Unpack the binary and move its files to your plugin directory. 

4. Start Vault by using the [server](https://www.vaultproject.io/docs/commands/server) command. Example in a dev environment:

	```bash
	vault server -dev -dev-root-token-id=root -dev-plugin-dir=./vault/plugins -log-level=debug
	```

	**NOTE:** This example is for development environments. Do not start the HashiCorp Vault server in `-dev` mode in production.

	You may need to also set up the following environment variable:

	```bash
	export VAULT_ADDR='http://127.0.0.1:8200'
	```

5. Get the SHA-256 checksum of the plugin binary:

	```bash
	SHA256=$(sha256sum /private/etc/vault/plugins/vault-plugin-secrets-datastax-astra| cut -d' ' -f1)
	```

6. Register the `vault-plugin-secrets-datastax-astra` plugin in the Vault system catalog:

	```bash
	vault write sys/plugins/catalog/secret/vault-plugin-secrets-datastax-astra \
	sha_256="${SHA256}" command="vault-plugin-secrets-datastax-astra"
	```

	**Output:**
	```bash
	Success! Data written to: sys/plugins/catalog/secret/vault-plugin-secrets-datastax-astra
	```

7. Enable the plugin in your Vault instance:

	```bash
	vault secrets enable -path=astra vault-plugin-secrets-datastax-astra
	```
	**Output:**
	```bash
	Success! Enabled the vault-plugin-secrets-datastax-astra secrets engine at: astra/
	```

At this point, HashiCorp Vault and Astra DB Plugin for HashiCorp Vault are set up. They're ready to use.

## Using Astra DB tokens with HashiCorp Vault

There are several tasks you can submit with HashiCorp Vault commands:

* Add a root token for each Astra DB organization
* Read and list configurations
* Generate HashiCorp Vault roles from Astra DB roles
* Generate new tokens and attach meaningful, custom metadata for your company's tracking and auditing purposes

In this example, assume a company has three Astra DB organizations:

* A retail org
* A wholesale org
* An internal usage org

Follow these steps:

1. Add a root token for each Astra DB organization. Format:

	```bash
	vault write astra/config org_id="<ORG ID>" astra_token="<YOUR ASTRA ADMINISTRATOR APP TOKEN>" \
 	 url="https://api.astra.datastax.com" logical_name="<YOUR LOGICAL NAME>"
	```

	**TIP:** To get your `astra_token` value, in [Astra DB console](https://astra.datastax.com), login and go to Organization Settings > Token Management > Select Role: Organization Administrator. Click **Generate Token**. Copy the generated token from the resulting dialog. In the following example, the ID values have been obfuscated:

	![Sample UI with generated but obfuscated token value](images/astra-db-plugin-hashi-vault-generated-token3.png)

	Here's an example `vault` command to create a root token for the first organization:

	```bash
	vault write astra/config org_id="ccd999999_facd_4ad3_bbb99903d999999999999999d" astra_token="AstraCS:ONqZCOAAAAAAAAAAAAAAAAe:608ba9999999999999190219" \
	 url="https://api.astra.datastax.com" logical_name="retailOrg"
	```
	**Output:**
	```bash
	Success! Data written to astra/configs
	```

	The created root token will be used by HashiCorp Vault for further token operations within this organization. 

	Submit a `vault write astra/config ...` command for each organization by providing its unique identifiers. Remember to also specify a unique `logical_name` value, such as `logical_name="wholesaleOrg"`.  Examples:

	```bash
	vault write astra/config org_id="Some0therOrgId_aaa999999_bbbb_4ad3_ccc99903d" astra_token="AstraCS:Some0therUniqueTokenF0rThisOrg999" \
	 url="https://api.astra.datastax.com" logical_name="wholesaleOrg"
	```

	And:

	```bash
	vault write astra/config org_id="Y3tAnotherOrgId_aaa777777_bbbb_4ad3_ccc77777d" astra_token="AstraCS:YetAn0therUniqueTokenF0rThisOrg777" \
	 url="https://api.astra.datastax.com" logical_name="internalOrg"
	```

2. List the created organization/token configurations:

	```bash
	vault list astra/configs
	```

	**Sample output:**
	```bash
	config/ccd999999_facd_4ad3_bbb99903d999999999999999d
	config/Some0therOrgId_aaa999999_bbbb_4ad3_ccc99903dd
	config/Y3tAnotherOrgId_aaa777777_bbbb_4ad3_ccc77777d
	```

	Referring to the listed IDs, you can then submit read operation to get the defined properties. Example searching by `org_id`:

	```bash
	vault read astra/config org_id="ccd999999_facd_4ad3_bbb99903d999999999999999d"
	```

	**Sample output:**

	```bash
	Key                 Value
 	---                 -----
 	astra_token         AstraCS:ONqZCOkoDjGmDhEwJLiCvsSe:608ba0291db907bc45d5c190219
	logical_name        InternalOrg
	org_id              Y3tAnotherOrgId_aaa777777_bbbb_4ad3_ccc77777d
	url                 https://api.astra.datastax.com
	```

	You can also use the `vault read astra/config...` command to search by `logical_name`.

3. Use the installed token to automatically generate HashiCorp Vault roles from Astra DB roles:

	You can get a list of `role_id` values for an Astra DB organization by using the DataStax DevOps API. Example:

	```bash
	curl --request GET \
	 --url 'https://api.astra.datastax.com/v2/organizations/roles' \
	 --header 'Accept: application/json' \
	 --header 'Authorization: Bearer <application_token>'
	```

	Or you can run the [update_roles.sh](https://github.com/datastax/vault-plugin-secrets-datastax-astra/blob/main/update_roles.sh) script. It's provided in our GitHub repo. The script adds all the Astra DB roles (default and custom) and their IDs to HashiCorp Vault. Example:

	```bash
	sh vault/plugins/vault-plugin-secrets-datastax-astra/update_roles.sh
	```

4. List the roles created across all your Astra DB organizations:

	```bash
	vault list astra/roles
	```

	You can also return the metadata for a specific role. Example:

	```bash
	vault read astra/role org_id="<ORG ID>" role="<ROLE NAME>"
	```

	Also available is the `vault delete astra/role org_id="<ORG ID>" role="<ROLE NAME>"` command.

5. For any of the roles, you can use HashiCorp Vault to generate a new Astra DB token. Example:

	```bash
	vault write astra/org/token org_id="<ORG ID>" role_name="<ROLE NAME>"
	```

	**TIP:** You can also apply custom, meaningful metadata to the generated Astra DB token by adding one or more `metadata` parameters. The metadata names and values can be any free-form text that you want. Example:

	```bash
	vault write astra/org/token org_id="ccd999999_facd_4ad3_bbb99903d999999999999999d" role_name="organization_administrator" \
	 metadata="user=mrsmart" metadata="purpose=demo"
	```

	The command output displays the new token's properties, including:

	* `clientId` (you'll see this value in Astra DB console too)
	* `generatedOn` (date format)
	* `metadata` (example: `map[purpose=demo user=mrsmart]`)
	* `orgId` (its value)
	* `token` (example: its `AstraCS:<generated-token-id>` value)

	With the newly generated token, you can now make calls to Astra DB via its APIs.

6. You can also delete tokens. Example:

	```bash
	vault delete astra/org/token org_id="ccd999999_facd_4ad3_bbb99903d999999999999999d" role_name="organization_administrator"
	```

	**Output:**
	```
	Success! Data deleted (if it existed) at: astra/org/token
	```

## Summary

HashiCorp Vault has a full understanding of the historical token specifics, for control and auditing purposes, including when the tokens were used and by whom, along with a free-form role name and any custom metadata you may have associated with the tokens. For example, in the example above, HashiCorp Vault's data knows the details of the token delete operation via its identity management and access control data; whereas Astra DB (in this example) is only aware that a token of a particular clientId was generated on a date, and has since been deleted.

## Community contributions

Astra DB Plugin for HashiCorp Vault is an open source project. In this GitHub repo, use [Issues](https://github.com/datastax/vault-plugin-secrets-datastax-astra/issues) to report a problem or share an idea. You may suggest ideas for improvement or bug fixes. [Clone the repo](https://github.com/datastax/vault-plugin-secrets-datastax-astra) and submit a Pull Request (PR) on a separate fork and working branch. This OSS project is a community effort - we encourage and appreciate contributions!

## What's next

See the following resources:

* [Video introduction](https://youtu.be/_NUK6-omsyA) on YouTube
* [HashiCorp Vault](https://www.hashicorp.com/products/vault) documentation
* [How to generate tokens in Astra DB](https://docs.datastax.com/en/astra/docs/manage/org/managing-org.html#_manage_application_tokens) 
* [Astra DB user permissions](https://docs.datastax.com/en/astra/docs/manage/org/user-permissions.html)
