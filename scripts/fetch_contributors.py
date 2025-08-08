#!/usr/bin/env python3
"""
Script to fetch contributors from GitHub and save them as a static JSON file.
This script is meant to be run during the documentation build process.
"""

import json
import os
import re
import sys
import urllib.request
import urllib.error

# Repository information
REPO_OWNER = "truefoundry"
REPO_NAME = "KubeElasti"
OUTPUT_PATH = "docs/assets/contributors.json"

# Bot patterns to filter out
BOT_PATTERNS = [
    r"-bot$",
    r"-automation$",
    r"\[bot\]$",
    r"^dependabot",
    r"^renovate",
    r"^github-actions",
    r"^semantic-release",
    r"^imgbot",
    r"^codecov",
    r"^snyk",
    r"^greenkeeper",
    r"^depfu",
    r"^pyup-bot",
]

def is_bot(username):
    """Check if a username matches any bot pattern."""
    return any(re.search(pattern, username, re.IGNORECASE) for pattern in BOT_PATTERNS)

def fetch_contributors():
    """Fetch contributors from GitHub API."""
    url = f"https://api.github.com/repos/{REPO_OWNER}/{REPO_NAME}/contributors"
    
    # Use GitHub token if available
    headers = {}
    github_token = os.environ.get("GITHUB_TOKEN")
    if github_token:
        headers["Authorization"] = f"token {github_token}"
    
    try:
        request = urllib.request.Request(url, headers=headers)
        with urllib.request.urlopen(request) as response:
            if response.status != 200:
                print(f"Error: Received status code {response.status}", file=sys.stderr)
                return None
            
            return json.loads(response.read().decode())
    except urllib.error.URLError as e:
        print(f"Error fetching contributors: {e}", file=sys.stderr)
        return None

def filter_and_simplify_contributors(contributors):
    """Filter out bots and simplify contributor data."""
    if not contributors:
        return []
    
    human_contributors = []
    for contributor in contributors:
        if is_bot(contributor["login"]):
            continue
        
        # Only keep necessary fields to reduce file size
        human_contributors.append({
            "login": contributor["login"],
            "avatar_url": contributor["avatar_url"],
            "html_url": contributor["html_url"]
        })
    
    return human_contributors

def main():
    """Main function to fetch and save contributors."""
    print("Fetching contributors from GitHub...")
    contributors = fetch_contributors()
    
    if not contributors:
        print("Failed to fetch contributors", file=sys.stderr)
        return 1
    
    human_contributors = filter_and_simplify_contributors(contributors)
    print(f"Found {len(human_contributors)} human contributors")
    
    # Create directory if it doesn't exist
    os.makedirs(os.path.dirname(OUTPUT_PATH), exist_ok=True)
    
    # Save to JSON file
    with open(OUTPUT_PATH, "w") as f:
        json.dump(human_contributors, f)
    
    print(f"Contributors saved to {OUTPUT_PATH}")
    return 0

if __name__ == "__main__":
    sys.exit(main())
