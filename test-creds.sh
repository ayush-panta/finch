#!/bin/bash
set -e

echo "Testing credential bridge with concurrent operations..."

# # Ensure ECR login
# export AWS_ACCOUNT_ID=299170649678
# export AWS_REGION=us-west-2
# aws ecr get-login-password --region $AWS_REGION | ./_output/bin/finch login --username AWS --password-stdin $AWS_ACCOUNT_ID.dkr.ecr.$AWS_REGION.amazonaws.com

# Test 1: Build with private base image (tests credential access during build)
echo "Building image with private base..."
./_output/bin/finch build -f Dockerfile.test-creds -t test-creds-image .

# Test 2: Tag with multiple names
echo "Tagging image with multiple names..."
./_output/bin/finch tag test-creds-image $AWS_ACCOUNT_ID.dkr.ecr.$AWS_REGION.amazonaws.com/test:A
./_output/bin/finch tag test-creds-image $AWS_ACCOUNT_ID.dkr.ecr.$AWS_REGION.amazonaws.com/test:B  
./_output/bin/finch tag test-creds-image $AWS_ACCOUNT_ID.dkr.ecr.$AWS_REGION.amazonaws.com/test:C
./_output/bin/finch tag test-creds-image $AWS_ACCOUNT_ID.dkr.ecr.$AWS_REGION.amazonaws.com/test:D
./_output/bin/finch tag test-creds-image $AWS_ACCOUNT_ID.dkr.ecr.$AWS_REGION.amazonaws.com/test:E

# Test 3: Push all concurrently (stress test credential bridge)
echo "Pushing all images concurrently..."
./_output/bin/finch push $AWS_ACCOUNT_ID.dkr.ecr.$AWS_REGION.amazonaws.com/test:A &
./_output/bin/finch push $AWS_ACCOUNT_ID.dkr.ecr.$AWS_REGION.amazonaws.com/test:B &
./_output/bin/finch push $AWS_ACCOUNT_ID.dkr.ecr.$AWS_REGION.amazonaws.com/test:C &
./_output/bin/finch push $AWS_ACCOUNT_ID.dkr.ecr.$AWS_REGION.amazonaws.com/test:D &
./_output/bin/finch push $AWS_ACCOUNT_ID.dkr.ecr.$AWS_REGION.amazonaws.com/test:E &

# Wait for all pushes to complete
wait
echo "✅ All credential operations completed successfully!"

# Cleanup
echo "Cleaning up test images..."
./_output/bin/finch image rm test-creds-image
./_output/bin/finch image rm $AWS_ACCOUNT_ID.dkr.ecr.$AWS_REGION.amazonaws.com/test:A
./_output/bin/finch image rm $AWS_ACCOUNT_ID.dkr.ecr.$AWS_REGION.amazonaws.com/test:B
./_output/bin/finch image rm $AWS_ACCOUNT_ID.dkr.ecr.$AWS_REGION.amazonaws.com/test:C
./_output/bin/finch image rm $AWS_ACCOUNT_ID.dkr.ecr.$AWS_REGION.amazonaws.com/test:D
./_output/bin/finch image rm $AWS_ACCOUNT_ID.dkr.ecr.$AWS_REGION.amazonaws.com/test:E

echo "🎉 Credential bridge test completed!"