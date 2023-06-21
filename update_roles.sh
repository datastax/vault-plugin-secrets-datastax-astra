#!/bin/sh

#######
# Functions
#######

function get_org_ids() {
    ORGS=( $(vault list astra/configs  | tail -n +3) )
}

#function to get a list of roles
function get_roles_ids() {
    curl --location --request GET $ENDPOINT \
    --header "Authorization: Bearer $SECRET" |jq -c '.[] |.id,.name' | sed 'N;s/\n/,/'
}

#function to iterate through the roles and build an array of roles and names
function make_vault_roles() {
    IFS=$'\n'       # make newlines the only separator
    for this_org in `get_roles_ids`
    do
        ORG=$1
        ID=`echo $this_org | cut -d, -f1 |sed 's/"//g'`
        #Make the name safe for use in a vault path via last sed
        NAME=`echo $this_org | cut -d, -f2 |sed 's/"//g' |sed -E 's/[^[:alnum:]]+/_/g'`
        echo "Creating Role '$NAME' with UUID '$ID'"
        vault write astra/role role_id="$ID" org_id="$ORG" role_name="$NAME"
    done
}


#######
# Main
#######

#Genereate a list of orgs
get_org_ids

#Iterate through each organization
for ORG in "${ORGS[@]}"
do
    echo "Updating roles for org '$ORG'"

    #get the token for the organization
    SECRET=$(vault read -field astra_token astra/config org_id=$ORG)
    ENDPOINT=$(vault read -field url astra/config org_id=$ORG)
    ENDPOINT+='/v2/organizations/roles'
    echo $ENDPOINT

    #Create roles for the organization
    make_vault_roles $ORG
    
done





