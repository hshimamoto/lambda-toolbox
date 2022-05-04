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

# upload them
srcs=""
for i in $files; do
	echo $i
	curl $url -F file=@$i
	if [ "$srcs" != "" ]; then
		srcs="$srcs,"
	fi
	srcs="$srcs\"tmp/$i\""
done
# concat
cmd="s3.concat"
dest="code/$zip"
curl $url -H 'content-type: application/json' -d "{\"command\":\"$cmd\",\"destination\":\"$dest\",\"sources\":[$srcs]}"
cmd="lambda.update"
curl $url -H 'content-type: application/json' -d "{\"command\":\"$cmd\",\"function\":\"$fname\",\"zipfile\":\"$dest\"}"
rm -f $files
echo "done"
