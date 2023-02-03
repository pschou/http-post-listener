#!/bin/bash

echo Got file $1

md5=$( md5sum "$1" )
echo md5=$md5
