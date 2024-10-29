<script>
    import { onMount } from "svelte";

    let leaderboard = [];
    let currentPage = 1;
    let hasMore = true;

    async function getLeaderboard(page) {
        const response = await fetch(`/api/leaderboard?page=${page}`);
        if (response.ok) {
            const data = await response.json();
            leaderboard = data;
            hasMore = data.length === 50;
        } else {
            alert("Failed to get leaderboard.");
        }
    }

    function nextPage() {
        if (hasMore) {
            currentPage++;
            getLeaderboard(currentPage);
        }
    }

    function previousPage() {
        if (currentPage > 1) {
            currentPage--;
            getLeaderboard(currentPage);
        }
    }

    onMount(() => {
        getLeaderboard(currentPage);
    });
</script>

<div class="max-w-4xl mx-auto p-6">
    <h1 class="text-3xl font-bold mb-6 text-center">Leaderboard</h1>
    <table class="min-w-full bg-white rounded-lg shadow overflow-hidden">
        <thead class="bg-gray-800 text-white">
            <tr>
                <th class="py-3 px-4 text-left">Player</th>
                <th class="py-3 px-4 text-left">Pure Points</th>
                <th class="py-3 px-4 text-left">Evil Points</th>
                <th class="py-3 px-4 text-left">Net Alignment</th>
            </tr>
        </thead>
        <tbody class="text-gray-700">
            {#each leaderboard as player}
                <tr class="border-b">
                    <td class="py-3 px-4">{player.nickname}</td>
                    <td class="py-3 px-4">{player.pure_points}</td>
                    <td class="py-3 px-4">{player.evil_points}</td>
                    <td class="py-3 px-4">{player.net_alignment}</td>
                </tr>
            {/each}
        </tbody>
    </table>
    
    <div class="flex justify-center gap-4 mt-6">
        <button 
            class="px-4 py-2 bg-gray-800 text-white rounded-lg disabled:opacity-50"
            on:click={previousPage}
            disabled={currentPage === 1}>
            Previous
        </button>
        <span class="py-2">Page {currentPage}</span>
        <button 
            class="px-4 py-2 bg-gray-800 text-white rounded-lg disabled:opacity-50"
            on:click={nextPage}
            disabled={!hasMore}>
            Next
        </button>
    </div>
</div>
