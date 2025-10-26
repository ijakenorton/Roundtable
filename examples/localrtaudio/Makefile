.PHONY: echoingaudiopeer micliveaudio app connecttoapp buildechoingaudiopeer buildmicliveaudio buildapp buildconnecttoapp

## BUILD
#-----------------------------------------------------------#
buildall:  buildechoingaudiopeer buildmicliveaudio buildapp 

buildechoingaudiopeer:
	go build echoingaudiopeer/main.go

buildmicliveaudio:
	go build micliveaudio/main.go

buildapp:
	go build rtaudioapp/main.go

buildconnecttoapp:
	go build connecttoapp/main.go
#-----------------------------------------------------------#

## RUN
#-----------------------------------------------------------#
echoingaudiopeer:
	go run echoingaudiopeer/main.go  -configFilePath echoingaudiopeer/config.yaml

micliveaudio:
	go run micliveaudio/main.go  -configFilePath micliveaudio/config.yaml

app:
	go run rtaudioapp/main.go  -configFilePath rtaudioapp/config.yaml

connecttoapp:
	go run connecttoapp/main.go  -configFilePath connecttoapp/config.yaml
#-----------------------------------------------------------#

