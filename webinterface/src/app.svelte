<script>
  import Router, { link, location } from 'svelte-spa-router';
  import Play from './components/play.svelte';
  import Leaderboard from './components/leaderboard.svelte';
  import Register from './components/register.svelte';

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
      <a href="/" use:link class="sidebar-brand">🎮 DNS RP</a>
      <div class="sidebar-links">
        <a href="/play" use:link class="sidebar-link" class:active={$location === '/play' || $location === '/'}>
          🕹️ Play
        </a>
        <a href="/leaderboard" use:link class="sidebar-link" class:active={$location === '/leaderboard'}>
          🏆 Leaderboard
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
    background-color: #2c3e50;
    width: 200px;
    padding: 1rem;
  }

  .sidebar-content {
    display: flex;
    flex-direction: column;
    height: 100%;
  }

  .sidebar-brand {
    color: #ecf0f1;
    font-size: 1.5rem;
    font-weight: bold;
    text-decoration: none;
    margin-bottom: 2rem;
    text-align: center;
  }

  .sidebar-links {
    display: flex;
    flex-direction: column;
    gap: 1rem;
  }

  .sidebar-link {
    color: #ecf0f1;
    text-decoration: none;
    padding: 0.75rem 1rem;
    border-radius: 8px;
    transition: background-color 0.3s;
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }

  .sidebar-link:hover, .sidebar-link.active {
    background-color: #34495e;
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
      padding: 1rem 0;
    }

    .sidebar-content {
      flex-direction: row;
      justify-content: space-between;
      align-items: center;
      padding: 0 1rem;
    }

    .sidebar-brand {
      margin-bottom: 0;
    }

    .sidebar-links {
      flex-direction: row;
    }

    .main-content {
      padding: 1rem;
    }
  }
</style>
