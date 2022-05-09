#!/bin/bash

if [ $# -lt 3 ]; then
	echo "$0 <url> <function name> <handler.zip>"
	exit 1
fi

url=$1
fname=$2
zip=$3

# split 4M 4*1024*1024=4194304
prefix=$zip.
split -b 4194304 -d $zip $prefix

files=$(/bin/ls $prefix??)
echo $files
cnt=$(echo $files | wc -l)
echo $cnt

# upload them in /tmp
srcs=""
for i in $files; do
	echo $i
	curl $url -F tmp=@$i
	if [ "$srcs" != "" ]; then
		srcs="$srcs,"
	fi
	srcs="$srcs\"$i\""
done
# concat in /tmp
cmd="exec.concat"
dest="$zip"
req0="{\"command\":\"$cmd\",\"destination\":\"$dest\",\"sources\":[$srcs]}"
# upload to s3
cmd="s3.store"
dest="code"
srcs="\"$zip\""
req1="{\"command\":\"$cmd\",\"destination\":\"$dest\",\"sources\":[$srcs]}"
# finally lambda function update
cmd="lambda.update"
dest="code/$zip"
req2="{\"command\":\"$cmd\",\"function\":\"$fname\",\"zipfile\":\"$dest\"}"
curl $url -H 'content-type: application/json' -d "{\"requests\":[$req0,$req1,$req2]}"
rm -f $files
echo "done"
