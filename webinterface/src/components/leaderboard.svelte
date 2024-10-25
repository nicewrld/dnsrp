<!-- frontend/src/components/Leaderboard.svelte -->
<script>
    import { onMount } from "svelte";

    let leaderboard = [];

    async function getLeaderboard() {
        const response = await fetch("/api/leaderboard");
        if (response.ok) {
            leaderboard = await response.json();
        } else {
            alert("Failed to get leaderboard.");
        }
    }

    onMount(() => {
        getLeaderboard();
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
</div>
