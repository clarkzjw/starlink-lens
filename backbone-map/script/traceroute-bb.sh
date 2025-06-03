#!/bin/bash

location="your-city-name-country-name"

OUTPUT_FILE_SUBNET1="$location-starlink-backbone-traceroute-149.19.txt"
OUTPUT_FILE_SUBNET2="$location-starlink-backbone-traceroute-206.224.txt"

# Clear the output files at the beginning
>"$OUTPUT_FILE_SUBNET1"
>"$OUTPUT_FILE_SUBNET2"

for i in $(seq 108 109); do
    for j in $(seq 0 255); do
        echo "Tracing 149.19.$i.$j"
        traceroute -enm 18 -w 1 149.19.$i.$j >> "$OUTPUT_FILE_SUBNET1"
    done
done

for i in $(seq 64 95); do
    for j in $(seq 0 255); do
        echo "Tracing 206.224.$i.$j"
        traceroute -enm 18 -w 1 206.224.$i.$j >> "$OUTPUT_FILE_SUBNET2"
    done
done

echo "Starlink backbone traceroute completed. Results saved to $OUTPUT_FILE_SUBNET1 and $OUTPUT_FILE_SUBNET2."

