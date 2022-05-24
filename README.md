
# vault-plugin-secrets-datastax-astra

This HashiCorp Vault plugin provides token lifecycle management for DataStax Astra DB. Due to the nature of the Astra DB object hierarchy, API tokens are not tied to users and do not currently have descriptions. It's easy to lose track of what tokens are being used for what purpose and who is using them. HashiCorp Vault allows you to associate metadata with a token (such as user who created it, what it's being users for) and additionally logs who's access the tokens to ensure token ownership and usage can be well understood. The DataStax Astra DB plugin for HashiCorp Vault functionality includes:

-   Use HashiCorp Vault to log access to Astra DB tokens.
    
-   Use HashiCorp Vault to create and revoke Astra DB tokens.
    
-   Use HashiCorp Vault to associated metadata to Astra DB tokens for tracking token ownership/application.
    
-   Use HashiCorp Vault to rotate tokens (future).

For more, see the expanded [docs topic](docs/index.md).

**Prerequisites**

 - You must have a functional HashiCorp Vault instance configured and the "vault" command must be usable. 
 
 - [Golang](https://go.dev/doc/install) must be installed.
 
 - You must have an Astra DB account.
 
 - You must have admin access to an Astra DB organization.
   
 - A "root token" must be created for each organization that HashiCorp Vault will manage. These will be the tokens that HashiCorp Vault uses to generate additional tokens. These root tokens must be created with the appropriate permission to create new tokens. For details on how to generate tokens, see [Manage application tokens](https://docs.datastax.com/en/astra/docs/manage/org/manage-tokens.html) in the Astra DB documentation. 

**Install the vault plugin into an existing vault**

Download the plugin binary for your OS from:

https://github.com/datastax/vault-plugin-secrets-datastax-astra/releases/tag/v0.1.0

Ensure the plugin is installed in the vault/plugins/vault-plugin-secrets-datastax-astra folder.

Or, build the plugin from source.

    go build -o vault/plugins/vault-plugin-secrets-datastax-astra cmd/vault-plugin-secrets-datastax-astra/main.go

Enable the plugin in your vault instance.

    vault secrets enable -path=astra vault-plugin-secrets-datastax-astra

**Configure the plugin**

Add root tokens for each Astra DB org:

    vault write astra/config org_id="<YOUR ORG ID>" astra_token="<YOUR APP TOKEN>" url="https://api.astra.datastax.com" logical_name="<YOUR LOGICAL NAME>"

List the created org/token configs:

    vault list astra/configs

Use the installed token to automatically generate Vault roles from Astra DB roles:

     sh vault/plugins/vault-plugin-secrets-datastax-astra/update_roles.sh

List the roles created:

    vault list astra/roles

Use Vault to create a new Astra DB token:

    vault write astra/org/token org_id=<YOUR ORG ID> role_name="<YOUR ROLE NAME>"

## Next steps

For more, see the expanded [docs topic](docs/index.md).
