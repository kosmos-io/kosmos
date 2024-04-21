#!/bin/bash

# 定义转换函数
convert_to_json() {
    local input="$1"
    local output="{"

    local pair_count=$(echo $input | wc -w)

    count=0
    for pair in $input; do
        key=$(echo "$pair" | cut -d'=' -f1)
        value=$(echo "$pair" | cut -d'=' -f2)
        output+="\"$key\": \"$value\""

        count=$((count+1))

        if [ $count -lt $pair_count ]; then
            output+=", "
        fi

    done

   # output="${output%,}"
    output+="}"
    echo "$output"
}