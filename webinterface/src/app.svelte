<script>
  import { fade } from 'svelte/transition';
  let showAbout = false;

  import Router from 'svelte-spa-router';
  import { link, location } from 'svelte-spa-router';
  import Play from './components/play.svelte';
  import Leaderboard from './components/leaderboard.svelte';
  import Register from './components/register.svelte';
  import About from './components/about.svelte';
  import { PlayIcon, TrophyIcon, InfoIcon } from 'lucide-svelte';


  const routes = {
    '/': Play,
    '/play': Play,
    '/leaderboard': Leaderboard,
    '/register': Register,
    '/about': About,

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
        <a href="/about" use:link class="sidebar-tab" class:active={$location === '/about'} role="tab">
          <InfoIcon size={24} />
          <span class="sidebar-tab-text">About</span>
        </a>

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
