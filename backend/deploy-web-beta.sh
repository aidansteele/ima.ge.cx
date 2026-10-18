#!/bin/bash
# Run from backend/. Deploys only the parallel beta API, never the workers/CDN.
set -euo pipefail
export AWS_PROFILE="${AWS_PROFILE:-imagecx}"
export AWS_REGION=us-west-2

if [[ $(aws sts get-caller-identity --query Account --output text) != 798327088984 ]]; then
  echo 'Refusing to deploy outside the imagecx account.' >&2
  exit 1
fi

build="$PWD/.aws-sam/web"
mkdir -p "$build/package/bin" "$build/package/licenses"
# Build the static ARM64 decoder from the same pinned source as Event Functions.
docker buildx build --platform linux/arm64 --target web-api \
  --output "type=local,dest=$build/decoder" targzi
cp "$build/decoder/bin/gztool" "$build/package/bin/"
cp "$build/decoder/licenses/"* "$build/package/licenses/"
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -buildvcs=false \
  -ldflags='-s -w -buildid=' -o "$build/package/server" ./thelambda
cp thelambda/index.js "$build/package/index.js"
# Re-create the archive so removed files cannot survive a later deployment.
rm -f "$build/api.zip"
(cd "$build/package" && zip -qr "$build/api.zip" index.js server bin licenses)
hash=$(shasum -a 256 "$build/api.zip" | cut -d ' ' -f 1)

backend=$(aws cloudformation describe-stacks --stack-name imagecx-backend-beta)
output() { jq -er --arg key "$1" '.Stacks[0].Outputs[] | select(.OutputKey == $key) | .OutputValue'; }
table=$(output Table <<< "$backend")
bucket=$(output Bucket <<< "$backend")
machine=$(output Machine <<< "$backend")
stack=imagecx-api-web-beta
parameters=("TableName=$table" "DataBucket=$bucket" "MachineArn=$machine")

# Bootstrap the versioned code bucket only on the first deployment. An existing
# endpoint stays on its current revision until the complete new revision is ready.
if ! code_bucket=$(aws cloudformation describe-stacks --stack-name "$stack" | output CodeBucket); then
  aws cloudformation deploy --stack-name "$stack" --template-file web-function.yml \
    --capabilities CAPABILITY_IAM --no-fail-on-empty-changeset \
    --parameter-overrides "${parameters[@]}"
  code_bucket=$(aws cloudformation describe-stacks --stack-name "$stack" | output CodeBucket)
fi
version=$(aws s3api put-object --bucket "$code_bucket" --key api.zip \
  --body "$build/api.zip" --query VersionId --output text)
aws cloudformation deploy --stack-name "$stack" --template-file web-function.yml \
  --capabilities CAPABILITY_IAM --no-fail-on-empty-changeset \
  --parameter-overrides "${parameters[@]}" "CodeVersion=$version" "CodeHash=$hash"
aws cloudformation describe-stacks --stack-name "$stack" --query 'Stacks[0].Outputs'
