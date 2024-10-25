<!-- frontend/src/components/register.svelte -->
<script>
    import { push } from "svelte-spa-router";
    let nickname = "";

    async function register() {
        const res = await fetch("/api/register", {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ nickname }),
        });
        if (res.ok) {
            push("/play");
        } else {
            const errorText = await res.text();
            alert(`Registration failed: ${errorText}`);
        }
    }
</script>

<div class="flex items-center justify-center min-h-screen bg-gray-100">
    <form
        on:submit|preventDefault={register}
        class="bg-white p-6 rounded shadow-md"
        style="width: 300px;"
    >
        <h1 class="text-2xl font-bold mb-4 text-center">Register</h1>
        <div class="mb-4">
            <label class="block text-gray-700 mb-2" for="nickname"
                >Nickname</label
            >
            <input
                id="nickname"
                type="text"
                bind:value={nickname}
                required
                class="w-full px-3 py-2 border rounded focus:outline-none focus:ring focus:border-blue-300"
            />
        </div>
        <button
            type="submit"
            class="w-full bg-blue-500 text-white py-2 rounded hover:bg-blue-600 focus:outline-none focus:ring"
        >
            Register
        </button>
    </form>
</div>
