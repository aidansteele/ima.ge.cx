#!/bin/sh
set -eux
export CGO_ENABLED=0
export GOFLAGS="-buildvcs=false -trimpath '-ldflags=-s -w -buildid='"

#cd next
#npm run build
#npx next export -o ../thelambda/frontend
#cd ..
#mv thelambda/frontend/[[...index]].html thelambda/frontend/index.html

sam build --parallel

sam deploy \
  --stack-name imagecx-backend-beta \
  --resolve-s3 \
  --capabilities CAPABILITY_IAM CAPABILITY_AUTO_EXPAND \
  --no-fail-on-empty-changeset

# stackit up --stack-name imagecx-backend --template infra.yml Domain=beta-api.ima.ge.cx Origin=xelq4iwrc3cx6vngdq6cbc3ewy0djzvf.lambda-url.us-west-2.on.aws
