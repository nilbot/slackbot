#!/bin/bash

go get -u github.com/nilbot/slackbot/app/hackernews
export GOOGLE_APPLICATION_CREDENTIALS=~/cred.json
kill -9 $(ps ux | grep '[h]ackernews ' | awk '{print $2}')
token=$(cat ~/.slackbot)
hackernews "$token" >> ~/.hackernews.log 2>&1 &
disown
