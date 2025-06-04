#!/bin/bash
# call this script with an email address (valid or not).
mkdir ./c_example/go_client/certs
rm ./c_example/go_client/certs/*
echo "make server cert"
openssl req -new -nodes -x509 -out ./go_client/certs/server.pem -keyout ./go_client/certs/server.key -days 3650 -subj "/C=DE/ST=NRW/L=Earth/O=Random"
echo "make client cert"
openssl req -new -nodes -x509 -out ./go_client/certs/client.pem -keyout ./go_client/certs/client.key -days 3650 -subj "/C=DE/ST=NRW/L=Earth/O=Random"
