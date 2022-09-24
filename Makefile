.PHONY: install

fotografiska: *.go
	go build -buildvcs=false -o fotografiska .

install: fotografiska
	cp fotografiska /usr/local/bin/fotografiska
