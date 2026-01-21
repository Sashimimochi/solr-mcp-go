#!/bin/bash

# Docker Build and Push Script for solr-mcp-go
# Usage: ./docker-build-push.sh [version]

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
DOCKER_USERNAME="${DOCKER_USERNAME:-343mochi}"  # Replace with your DockerHub username
IMAGE_NAME="solr-mcp-go"
VERSION="${1:-latest}"

echo -e "${GREEN}=== Docker Build and Push Script ===${NC}"
echo "Username: ${DOCKER_USERNAME}"
echo "Image: ${IMAGE_NAME}"
echo "Version: ${VERSION}"
echo ""

# Check if Docker is running
if ! docker info > /dev/null 2>&1; then
    echo -e "${RED}Error: Docker is not running${NC}"
    exit 1
fi

# Build the Docker image
echo -e "${YELLOW}Building Docker image...${NC}"
docker build -t "${DOCKER_USERNAME}/${IMAGE_NAME}:${VERSION}" .

# Tag as latest if a version is specified
if [ "${VERSION}" != "latest" ]; then
    echo -e "${YELLOW}Tagging as latest...${NC}"
    docker tag "${DOCKER_USERNAME}/${IMAGE_NAME}:${VERSION}" "${DOCKER_USERNAME}/${IMAGE_NAME}:latest"
fi

echo -e "${GREEN}Build completed successfully!${NC}"
echo ""

# Ask for confirmation before pushing
read -p "Do you want to push to DockerHub? (y/n) " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    # Check if logged in to DockerHub
    if ! docker info | grep -q "Username: ${DOCKER_USERNAME}"; then
        echo -e "${YELLOW}Logging in to DockerHub...${NC}"
        docker login
    fi

    # Push the image
    echo -e "${YELLOW}Pushing image to DockerHub...${NC}"
    docker push "${DOCKER_USERNAME}/${IMAGE_NAME}:${VERSION}"

    if [ "${VERSION}" != "latest" ]; then
        docker push "${DOCKER_USERNAME}/${IMAGE_NAME}:latest"
    fi

    echo -e "${GREEN}Push completed successfully!${NC}"
    echo ""
    echo "Image available at:"
    echo "  docker pull ${DOCKER_USERNAME}/${IMAGE_NAME}:${VERSION}"
    if [ "${VERSION}" != "latest" ]; then
        echo "  docker pull ${DOCKER_USERNAME}/${IMAGE_NAME}:latest"
    fi
    echo ""
    echo -e "${YELLOW}Next steps:${NC}"
    echo "1. Update DockerHub repository description:"
    echo "   - Go to: https://hub.docker.com/r/${DOCKER_USERNAME}/${IMAGE_NAME}"
    echo "   - Click 'Edit' on the repository page"
    echo "   - Copy content from DOCKER_README.md to the 'Full Description' field"
    echo "   - Update GitHub link in 'Repository links' section"
    echo ""
    echo "2. To update description via CLI (requires dockerhub-description tool):"
    echo "   npm install -g dockerhub-description"
    echo "   dockerhub-description ${DOCKER_USERNAME}/${IMAGE_NAME} DOCKER_README.md"
else
    echo -e "${YELLOW}Skipping push to DockerHub${NC}"
fi

echo ""
echo -e "${GREEN}Done!${NC}"
