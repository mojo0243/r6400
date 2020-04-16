# Go Parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOGET=$(GOCMD) get
BINARY_SERVER=server
BINARY_CLIENT=client
BINARY_SHELL=shell
FLAGS="-s -w"

# Client Settings
URL=http://192.168.1.3:8080/tasking
SECRET=work
CALLS=5
RING=1
TARG=n5

LDFLAGS="-s -w -X main.targ=$(TARG) -X main.url=$(URL) -X main.calls=$(CALLS) -X main.ring=$(RING) -X main.secret=$(SECRET)"

build_server:
	cd server; $(GOBUILD) -o $(BINARY_SERVER) -v -ldflags $(FLAGS); cd ../

build_shell:
	cd shell; $(GOBUILD) -o $(BINARY_SHELL) -v -ldflags $(FLAGS); cd ../

build_r6400:
	cd client; GOOS=linux GOARCH=arm $(GOBUILD) -o $(TARG)_netgear -v -ldflags $(LDFLAGS); cd ../

build_linux64:
	cd client; GOOS=linux GOARCH=amd64 $(GOBUILD) -o $(TARG)_linux64 -v -ldflags $(LDFLAGS); cd ../

deps:
	$(GOGET) gopkg.in/yaml.v2
	$(GOGET) github.com/lib/pq
	$(GOGET) github.com/c-bata/go-prompt
	$(GOGET) github.com/common-nighthawk/go-figure
	$(GOGET) github.com/jedib0t/go-pretty/table
