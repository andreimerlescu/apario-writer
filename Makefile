.PHONY: install build run dbuild drun dbash containered

PROJECT = apario-contribution
TAG = go-1.20.4-buster

LOGFILE = logs/containered.log

install:
	go mod download

build:
	go build -a -race -v -o $(PROJECT) .

test: build
	time ./$(PROJECT) -dir tmp -file importable/sample-jfk.xlsx -limit 999 -buffer 999666333

run: build
	./$(PROJECT) -dir tmp -file importable/$(filter-out $@,$(MAKECMDGOALS)) -limit 999 -buffer 999666333

containered:
	rm -f $(LOGFILE)
	touch $(LOGFILE)
	./$(PROJECT) -dir tmp -file importable/$(filter-out $@,$(MAKECMDGOALS)) -limit 33 -buffer 454545 -pdfcpu 1 -gs 1 -pdftotext 1 -convert 1 -pdftoppm 1 -png2jpg 1 -resize 1 -shafile 1 -watermark 1 -darkimage 1 -filedata 3 -shastring 3 -wjsonfile 3 -log ./$(LOGFILE) &
	PID=$$!
	trap 'kill $$TAIL_PID' EXIT
	tail -f $(LOGFILE) & TAIL_PID=$$!
	wait $$PID

dbuild:
	docker build -t $(PROJECT):$(TAG) .

drun:
	docker run -v tmp:/app/tmp -v logs:/app/logs $(PROJECT):$(TAG) make containered $(filter-out $@,$(MAKECMDGOALS))

dbash:
	docker run -t -i --rm -v tmp:/app/tmp -v logs:/app/logs $(PROJECT):$(TAG) bash