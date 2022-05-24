
# vault-plugin-secrets-datastax-astra

This Hashi Vault plugin provides for token lifecycle management for DataStax Astra DBaaS solution. Due to the nature of the Astra object hierarchy,  Astra's API tokens are not tied to users and do not currently have descriptions. It’s easy to lose track of what tokens are being used for what purpose and who is using them. Hashi Vault allows you to associate metadata with a token (such as user who created it, what it’s being users for) and additionally logs who’s access the tokens to ensure token ownership and usage can be well understood. The DataStax Astra plugin for Hashi Vault functionality includes:

-   Use Hashi Vault to log access to Astra tokens.
    
-   Use Hashi Vault to create and revoke Astra tokens.
    
-   Use Hashi Vault to associated metadata to Astra tokens for tracking token ownership/application.
    
-   Use Hashi Vault to rotate tokens (future).


**Prerequisites**

 - You must have a functional Hashi Vault instance configured and the "vault" command must be usable. 
 
 - [Golang](https://go.dev/doc/install) must be installed.
 
 - You must have an Astra Account.
 
 - You must have admin access to an Astra organization .
   
 - A "root token" must be created for each organization that Hashi Vault will manage. These will be the tokens that Hashi Vault uses to generate additional tokens. These root tokens must be created with the appropriate permission to create new tokens. See the DataStax Astra documentation on ["Mange Application Tokens"](https://docs.datastax.com/en/astra/docs/manage-application-tokens.html) for details on how to generate tokens.


**Install the vault plugin into an existing vault**

Download the plugin from ????? and ensure the plugin is installed in the vault/plugins/vault-plugin-secrets-datastax-astra folder.

Build the plugin.

    go build -o vault/plugins/vault-plugin-secrets-datastax-astra cmd/vault-plugin-secrets-datastax-astra/main.go


Enable the plugin in your vault instance.

    vault secrets enable -path=astra vault-plugin-secrets-datastax-astra

**Configure the plugin**

Add root tokens for each Astra org

    vault write astra/config org_id="<YOUR ORG ID>" astra_token="<YOUR APP TOKEN>" url="https://api.astra.datastax.com" logical_name="<YOUR LOGICAL NAME>"

List the created org/token configs

    vault list astra/configs

Use the installed token to automatically generate Vault roles from Astra Roles

     sh vault/plugins/vault-plugin-secrets-datastax-astra/update_roles.sh

List the roles created

    vault list astra/roles

Use Vault to create a new Astra token.

    vault write astra/org/token org_id=<YOUR ORG ID> role_name="<YOUR ROLE NAME>"

   
