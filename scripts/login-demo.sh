#!/bin/sh
set -eu

: "${CREATOR_PHONE:?set CREATOR_PHONE to an E.164 phone number}"

curl -fsS -X POST http://localhost:8080/login/code \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: creator-login-demo' \
  -d "{\"phone\":\"$CREATOR_PHONE\"}"

printf '\nCode: '
IFS= read -r CREATOR_CODE
: "${CREATOR_CODE:?enter the received code}"

curl -fsS -X POST http://localhost:8080/login/verify \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: creator-login-demo' \
  -d "{\"phone\":\"$CREATOR_PHONE\",\"code\":\"$CREATOR_CODE\",\"asset_id\":\"asset-2048\"}"
