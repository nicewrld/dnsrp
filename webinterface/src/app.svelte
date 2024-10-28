<script>
  import { fade } from 'svelte/transition';
  let showAbout = false;
  import Router from 'svelte-spa-router';
  import { link, location } from 'svelte-spa-router';
  import Play from './components/play.svelte';
  import Leaderboard from './components/leaderboard.svelte';
  import Register from './components/register.svelte';
  import { PlayIcon, TrophyIcon, InfoIcon, TwitterIcon, MailIcon } from 'lucide-svelte';

  const routes = {
    '/': Play,
    '/play': Play,
    '/leaderboard': Leaderboard,
    '/register': Register,
  };
</script>

<div class="app-container">
  <nav class="sidebar">
    <div class="sidebar-content">
      <a href="/" use:link class="sidebar-brand">dnsrp</a>
      <div class="sidebar-tabs" role="tablist">
        <a href="/play" use:link class="sidebar-tab" class:active={$location === '/play' || $location === '/'} role="tab">
          <PlayIcon size={24} />
          <span class="sidebar-tab-text">Play</span>
        </a>
        <a href="/leaderboard" use:link class="sidebar-tab" class:active={$location === '/leaderboard'} role="tab">
          <TrophyIcon size={24} />
          <span class="sidebar-tab-text">Leaderboard</span>
        </a>
      </div>
      <div class="mt-auto">
        <button 
          class="sidebar-about"
          on:click={() => showAbout = !showAbout}
        >
          <InfoIcon size={24} />
          <span class="sidebar-tab-text">About</span>
        </button>
        
        {#if showAbout}
          {#if showAbout}
            <div class="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50"
                 on:click={() => showAbout = false}
                 transition:fade>
              <div class="about-card bg-white dark:bg-gray-800 p-6 rounded-lg shadow-xl max-w-sm w-full mx-4"
                   on:click|stopPropagation>
                <h3 class="text-xl font-bold mb-4 text-gray-900 dark:text-white">Contact</h3>
                <div class="flex flex-col gap-3">
                  <a href="https://twitter.com/spuhghetti" 
                     target="_blank" 
                     rel="noopener noreferrer" 
                     class="about-link">
                    <TwitterIcon size={20} />
                    <span>@spuhghetti</span>
                  </a>
                  <a href="mailto:h@hhh.hn" 
                     class="about-link">
                    <MailIcon size={20} />
                    <span>h@hhh.hn</span>
                  </a>
                </div>
              </div>
            </div>
          {/if}
        {/if}
      </div>
    </div>
  </nav>

  <main class="main-content">
    <Router {routes} />
  </main>
</div>

<style>
  :global(body) {
    font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen-Sans, Ubuntu, Cantarell, 'Helvetica Neue', sans-serif;
    margin: 0;
    padding: 0;
    background-color: #f5f7fa;
    color: #333;
  }

  .app-container {
    display: flex;
    min-height: 100vh;
  }

  .sidebar {
    background: var(--sidebar-bg, #2c3e50);
    width: calc(var(--sidebar-width, 100px) + var(--sidebar-inner-padding, 16px) * 2);
    position: sticky;
    top: 0;
    height: 100vh;
  }

  .sidebar-content {
    display: flex;
    flex-direction: column;
    height: 100%;
    padding: var(--sidebar-inner-padding, 16px);
  }

  .sidebar-brand {
    color: var(--sidebar-highlight, #ecf0f1);
    font-size: 1.5rem;
    font-weight: bold;
    text-decoration: none;
    margin-bottom: 2rem;
    text-align: center;
  }

  .sidebar-tabs {
    display: flex;
    flex-direction: column;
    gap: var(--padding, 8px);
  }

  .sidebar-tab {
    display: flex;
    flex-direction: column;
    align-items: center;
    text-align: center;
    gap: 3px;
    padding: var(--padding, 8px) 3px;
    color: var(--sidebar-highlight, #ecf0f1);
    font-size: var(--sidebar-font-size, 14px);
    opacity: 0.75;
    text-decoration: none;
    border-radius: var(--border-radius, 8px);
    transition: background-color 0.2s, opacity 0.2s;
  }

  .sidebar-tab:hover, .sidebar-tab.active {
    background-color: var(--sidebar-hover, #34495e);
    opacity: 1;
  }

  .sidebar-tab.active {
    color: var(--sidebar-bg, #2c3e50);
    background: var(--sidebar-highlight, #ecf0f1);
  }

  .main-content {
    flex: 1;
    padding: 2rem;
    overflow-y: auto;
  }

  .sidebar-about {
    display: flex;
    flex-direction: column;
    align-items: center;
    text-align: center;
    gap: 3px;
    padding: var(--padding, 8px) 3px;
    color: var(--sidebar-highlight, #ecf0f1);
    font-size: var(--sidebar-font-size, 14px);
    opacity: 0.75;
    text-decoration: none;
    border-radius: var(--border-radius, 8px);
    transition: background-color 0.2s, opacity 0.2s;
    width: 100%;
    border: none;
    background: none;
    cursor: pointer;
  }

  .sidebar-about:hover {
    background-color: var(--sidebar-hover, #34495e);
    opacity: 1;
  }

  .about-card {
    transform: translateY(0);
    transition: transform 0.2s ease-out;
  }

  .about-link {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    padding: 0.75rem;
    color: #2c3e50;
    text-decoration: none;
    border-radius: 0.5rem;
    transition: all 0.2s ease;
  }

  .about-link:hover {
    background-color: rgba(0, 0, 0, 0.05);
    transform: translateY(-1px);
  }

  :global(.dark) .about-link {
    color: #e2e8f0;
  }

  :global(.dark) .about-link:hover {
    background-color: rgba(255, 255, 255, 0.05);
  }

  @media (max-width: 768px) {
    .app-container {
      flex-direction: column;
    }

    .sidebar {
      width: 100%;
      height: auto;
      position: static;
    }

    .sidebar-content {
      flex-direction: row;
      justify-content: space-between;
      align-items: center;
      padding: var(--sidebar-inner-padding, 16px);
    }

    .sidebar-brand {
      margin-bottom: 0;
    }

    .sidebar-tabs {
      flex-direction: row;
    }

    .sidebar-tab {
      flex-direction: row;
      padding: 5px var(--padding, 8px);
    }

    .main-content {
      padding: 1rem;
    }
  }
</style>
