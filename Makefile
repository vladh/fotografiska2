.PHONY: install

fotografiska:
	go build -o fotografiska .

install: fotografiska
	cp fotografiska /usr/local/bin/fotografiska
