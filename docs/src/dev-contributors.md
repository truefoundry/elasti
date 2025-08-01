---
title: Contributors
description: Contributors to the KubeElasti project
---

# Contributors

This page recognizes all the amazing people who have contributed to the KubeElasti project. We appreciate all contributions, from code to documentation, testing, and community support.

<br>

<div class="contributors-grid" id="contributors-grid">
  <!-- Contributors will be dynamically loaded here -->
  <p class="loading-message">Loading contributors...</p>
</div>

<script>
  document.addEventListener('DOMContentLoaded', async () => {
    try {
      // Fetch contributors from GitHub API
      const response = await fetch('https://api.github.com/repos/truefoundry/elasti/contributors');
      
      if (!response.ok) {
        throw new Error(`GitHub API responded with status: ${response.status}`);
      }
      
      const contributors = await response.json();
      
      // Clear loading message
      document.querySelector('#contributors-grid .loading-message').remove();
      
      // Filter out bot accounts
      const botPatterns = [
        /-bot$/i,                // ends with -bot
        /-automation$/i,        // ends with -automation
        /\[bot\]$/i,            // ends with [bot]
        /^dependabot/i,         // starts with dependabot
        /^renovate/i,          // starts with renovate
        /^github-actions/i,     // starts with github-actions
        /^semantic-release/i,   // starts with semantic-release
        /^imgbot/i,             // starts with imgbot
        /^codecov/i,            // starts with codecov
        /^snyk/i,               // starts with snyk
        /^greenkeeper/i,        // starts with greenkeeper
        /^depfu/i,              // starts with depfu
        /^pyup-bot/i,           // starts with pyup-bot
      ];
      
      const isBot = (username) => {
        return botPatterns.some(pattern => pattern.test(username));
      };
      
      // Filter and display human contributors only
      const humanContributors = contributors.filter(contributor => !isBot(contributor.login));
      
      // Process and display contributors
      humanContributors.forEach(contributor => {
        const contributorElement = createContributorElement(contributor);
        document.getElementById('contributors-grid').appendChild(contributorElement);
      });
      
      // Show message if no human contributors found (unlikely but just in case)
      if (humanContributors.length === 0) {
        const message = document.createElement('p');
        message.textContent = 'No contributors found.';
        document.getElementById('contributors-grid').appendChild(message);
      }
    } catch (error) {
      console.error('Error fetching contributors:', error);
      const errorMessage = document.createElement('p');
      errorMessage.textContent = 'Unable to load contributors. Please check back later.';
      
      document.getElementById('contributors-grid').innerHTML = '';
      document.getElementById('contributors-grid').appendChild(errorMessage);
    }
  });
  
  function createContributorElement(contributor) {
    const container = document.createElement('div');
    container.className = 'contributor';
    
    const link = document.createElement('a');
    link.href = contributor.html_url;
    link.target = '_blank';
    link.rel = 'noopener noreferrer';
    
    const avatar = document.createElement('img');
    avatar.src = contributor.avatar_url;
    avatar.alt = `${contributor.login}'s avatar`;
    avatar.loading = 'lazy';
    
    const name = document.createElement('div');
    name.className = 'contributor-name';
    name.textContent = contributor.login;
    
    link.appendChild(avatar);
    container.appendChild(link);
    container.appendChild(name);
    
    return container;
  }
</script>

<style>
  .contributors-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(100px, 1fr));
    gap: 15px;
    margin: 20px 0;
  }
  
  .contributor {
    display: flex;
    flex-direction: column;
    align-items: center;
    text-align: center;
    padding: 8px;
    border-radius: 6px;
    transition: transform 0.2s, box-shadow 0.2s;
  }
  
  .contributor:hover {
    transform: translateY(-3px);
    box-shadow: 0 3px 10px rgba(0, 0, 0, 0.1);
  }
  
  .contributor img {
    width: 60px;
    height: 60px;
    border-radius: 50%;
    object-fit: cover;
    margin-bottom: 8px;
  }
  
  .contributor-name {
    font-weight: bold;
    font-size: 0.9em;
  }
  
  .loading-message {
    grid-column: 1 / -1;
    text-align: center;
    padding: 20px;
  }
</style>
