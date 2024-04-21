#!/bin/bash
echo $1 $2
for ((COUNT = 1; COUNT <= $1; COUNT++)); do
  echo $COUNT
  sleep 1
done
