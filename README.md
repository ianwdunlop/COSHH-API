## COSHH API

This GOLang based application provides the backend API service to complement the [COSHH webapp](https://gitlab.mdcatapult.io/informatics/coshh/coshh-ui). It uses a Postgres Database to store information about chemicals, their hazardous properties, expiry dates and links to safety docs.

### Running

Copy the `.env_example` file to `.env`. Change the values to match your local setup. For example, point `COSHH_DATA_VOLUME` to where the 
`assets` data files are stored on your local machine. The `COSHH_DATA_VOLUME` is mounted into the docker container and the files are read from there. There is an example `labs.csv` file within the assets directory. You will also need a `projects.csv` and also a `usernames.txt` file if you are not using LDAP. There is an example usernames file within the testdata directory.

```bash
make run
```

This starts the API and a local Postgres instance.

By default, the app starts with no data. To populate the app, follow the [ETL guide](scripts/etl/README.md). You can also start with an empty db if you prefer. The Postgres docker container uses an [sql script](scripts/init.sql) to create the schema.

### LDAP & user names
Each chemical can have an owner and can read the list of possible owners from LDAP or a text file. If you have an LDAP service then you can use that by supplying the `LDAP_USERNAME` & `LDAP_PASSWORD` env vars and setting `LDAP_ENABLED` to false. If you don't have LDAP then you can supply a text file which contains each user name on a different line. Set the `USERNAME_FILE` env var to the path where the server can read the file from.

### Debugging
You can run it all using the docker-compose file (use within the `make run` task above) or start up your locally changed versions of the components using `docker compose up -d`. This will build your local version of the app and run it via docker.  
You can also run and debug it in your IDE of choice by starting the server using `cmd/main.go`.  
To run your local versions you need to tell the backend what database connection params to use and where the files detailing the different lab and project names are.


```bash
export DBNAME=informatics \
export HOST=localhost \
export PASSWORD=postgres \
export PORT=5432 \
export API_PORT=8080 \
export DBUSER=postgres \
export SCHEMA=coshh \
export LABS_CSV=<path/to/assets/labs.csv> \
export Auth0Audience="https://coshh-api-local.wopr.inf.mdc" \
export Auth0Domain="mdcatapult.eu.auth0.com" \
export LDAP_USER="coshhbind@medcat.local" \
export LDAP_PASSWORD=<copy password from 1Password> \
export LDAP_ENABLED=true
```

Note that in earlier versions of the code the db user was set using the env var `USER` but that tramples over the default on Linux/Mac. We have changed it to be `DBUSER` instead. 

`Auth0Audience` is the identifier used in the Auth0 setup page for the particular API within the chosen `Auth0Domain`.

LDAP username and password are used to get a list of users from the MDC LDAP server. The coshhbind@medcat.local user has
been created specifically for this purpose and has readonly access. The password is stored in 1Password.

Start the database (also seeds the db with initial data):
```bash
docker-compose up -d db
``` 

Start the API without docker:
```bash
cd cmd
go run main.go
```
OR

Start the API using docker:
```bash
docker compose up -d server
```

OR use your IDE and run the `cmd/main.go` file. Remember to set your env vars.

### Testing

`make test`

### Accessing the database locally from the command line

Ensure you set the schema, e.g.

```
psql -h localhost -U postgres -d informatics        \\ password is postgres
SET schema 'coshh';                                 
```

#### View the output of audit_triggers/trigger functions
This audit trigger/functions provide transactions records of CRUD operations done on the chemical table(informatics.coshh).
Before running the commands below. 
You must have inserted and updated data into the coshh schema tables, only then can you run the following commands

simply run this command 
```
`SELECT * FROM audit_coshh_logs;` or `SELECT * FROM audit_coshh_log_views;`

```
NB: If none of the commands work add a coshh prefix for example `coshh.audit_coshh_logs`.

### Testing Authenticated Routes
Get the Auth0 client token from the Auth0 web portal.
* Login in to the Auth0 web page.
* Switch to the correct Auth0 tenant.
* Go to `Applications` in the sidebar and select `APIs`. 
* Open the correct API page from the list and click on the `Test` tab. 
* Copy the access token from the `Response` box. 
* Use curl to auth against the example `protected` route.

```bash
curl --request GET \
  --url http:/localhost:8080/protected \
  --header 'authorization: Bearer INSERT AUTH0 TOKEN'
```  
Successful auth results in `"You have successfully authenticated"`. Failure to auth results in `{"message":"Requires authentication"}`.

### Gotchas

#### SQL

When writing any new sql queries always remember to commit the transaction!

#### Seed data
This is contained in /scripts/etl/init-003.sql and this script is run when the docker-compose file is run.
The data includes a NULL value for every column in the chemical table without a NON NULL constraint.  This is to help identify
any errors which might arise when deploying the UI - the live database contains some pretty shaky historic data.
The data includes one chemical which is archived, two which are expired and one which expires within 30 days (again, to
facilitate testing of the UI).

**Remember to run `docker-compose down` before running `make run` (or `docker-compose up`) if you want to re-seed the database.**

#### CI

There was a glitch in the publish API stage in CI in October 2022 (which has since resolved itself) which meant that in order to deploy the API, the image 
had to be  built locally and pushed up to the registry manually.  In the event this should happen again use this command:

```docker build -t registry.mdcatapult.io/informatics/software-engineering/coshh/api:<tag name> . && docker push registry.mdcatapult.io/informatics/software-engineering/coshh/api:<tag name>```

N.B Mac M1 users may need to build the image for amd64 (as opposed to arm64) with `--platform linux/amd64`

#### Debugging within VSCode

In order to tell VSCode to use the correct env vars you need to follow the following steps:

1) You will need to edit the `settings.json` for the project and add the following line 
```json
"go.testEnvFile": "${workspaceFolder}/test.env"
```
2) Then create a `test.env` file in the root of the project and add any env vars to it that you want to change from the defaults.  
For example:
```bash
LABS_CSV=/home/me/code/coshh/COSHH-API/assets/labs.csv
PROJECTS_CSV=/home/me/code/coshh/COSHH-API/assets/projects.csv
API_PORT=8080
CLIENT_ORIGIN_URL=http://localhost:4020
AUTH0_DOMAIN=me.eu.auth0.com
AUTH0_AUDIENCE=https://me-local.wopr.inf.mdc
AUTH0_CLIENT_ID=45yhdfmlknm3jkl45n35j
USERNAME_FILE=/home/me/code/coshh/COSHH-API/testdata/usernames.txt
```

### Licence

This project is licensed under the terms of the Apache 2 licence, which can be found in the repository as `LICENCE.txt`
