<!-- frontend/src/App.svelte -->
<script>
    import Router, { link } from 'svelte-spa-router';
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
  
  <nav class="bg-gray-800">
    <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
      <div class="flex items-center justify-between h-16">
        <div class="flex items-center">
          <a href="/play" use:link class="text-white text-xl font-bold">DNS Roleplay Game</a>
        </div>
        <div class="block">
          <div class="ml-4 flex items-center">
            <a href="/play" use:link class="text-gray-300 hover:text-white px-3 py-2 rounded-md text-sm font-medium">Play</a>
            <a href="/leaderboard" use:link class="text-gray-300 hover:text-white px-3 py-2 rounded-md text-sm font-medium">Leaderboard</a>
          </div>
        </div>
      </div>
    </div>
  </nav>
  
  <main class="py-6">
    <Router {routes} />
  </main>
  