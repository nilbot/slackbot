#!/bin/bash

go get -u -v github.com/nilbot/slackbot/app/hackernews
kill -9 $(ps ux | grep '[h]ackernews' | awk '{print $2}')
hackernew $(cat ~/.slackbot) 2>&1 > ~/.hackernews.log &
disown