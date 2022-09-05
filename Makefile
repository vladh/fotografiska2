.PHONY: install

fotografiska: *.go
	go build -o fotografiska .

install: fotografiska
	cp fotografiska /usr/local/bin/fotografiska
