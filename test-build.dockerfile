FROM 299170649678.dkr.ecr.us-west-2.amazonaws.com/test:plz

# Simple test to verify ECR credentials work during build
RUN echo "Build successful with ECR credentials"

# Tag and push test
# This Dockerfile tests:
# 1. FROM pulls from private ECR (requires credentials)
# 2. Build process works with credential socket
# 3. Can be tagged and pushed back to ECR