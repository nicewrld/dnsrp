<script>
    import { onMount } from "svelte";
    import { fade, fly } from "svelte/transition";
    import { push } from "svelte-spa-router";

    let dnsRequest = null;
    let selectedAction = "";
    let errorMessage = "";
    let submissionMessage = "";
    let countdown = 0;
    let retryDelay = 0;
    let countdownInterval;
    let isSubmitting = false;

    async function getDNSRequest() {
        try {
            const response = await fetch("/api/play");
            console.log("getDNSRequest response status:", response.status);

            if (response.ok) {
                dnsRequest = await response.json();
                console.log("Received DNS request:", dnsRequest);

                errorMessage = "";
                submissionMessage = "";
                selectedAction = "";

                clearInterval(countdownInterval);
                countdown = 0;
            } else if (response.status === 401) {
                push("/register");
            } else {
                errorMessage = "No DNS requests available.";
                retryDelay = getRandomInt(1000, 5000);
                countdown = retryDelay;
                startRetryCountdown();
            }
        } catch (error) {
            console.error("Error fetching DNS request:", error);
            errorMessage = "Failed to get DNS request.";
            retryDelay = getRandomInt(1000, 5000);
            countdown = retryDelay;
            startRetryCountdown();
        }
    }

    function startRetryCountdown() {
        clearInterval(countdownInterval);
        countdownInterval = setInterval(() => {
            countdown -= 10;
            if (countdown <= 0) {
                clearInterval(countdownInterval);
                getDNSRequest();
            }
        }, 10);
    }

    function getRandomInt(min, max) {
        return Math.floor(Math.random() * (max - min + 1)) + min;
    }

    async function submitAction() {
        if (isSubmitting) return;
        isSubmitting = true;
        const res = await fetch("/api/submit", {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({
                action: selectedAction,
                request_id: dnsRequest.request_id,
            }),
        });
        if (res.ok) {
            submissionMessage = "Action submitted successfully!";
            selectedAction = "";
            await new Promise((resolve) =>
                setTimeout(resolve, Math.random() * 1000),
            );
            await getDNSRequest();
        } else {
            const errorText = await res.text();
            submissionMessage = `Failed to submit action: ${errorText}`;
        }
        isSubmitting = false;
    }

    onMount(() => {
        getDNSRequest();
        return () => {
            clearInterval(countdownInterval);
        };
    });
</script>

<div class="h-full flex items-center justify-center px-4">
    <div class="max-w-2xl w-full p-6 bg-gray-800 rounded-lg shadow-lg text-white mx-auto">
    {#if dnsRequest}
        <div in:fly={{ y: 50, duration: 500 }} out:fade>
            <h1 class="text-3xl font-bold mb-6 text-center text-blue-400">
                Handle DNS Request
            </h1>
            <div class="bg-gray-700 p-4 rounded-lg mb-6">
                <p class="mb-2">
                    <span class="font-bold text-blue-300">Name:</span>
                    {dnsRequest.name}
                </p>
                <p class="mb-2">
                    <span class="font-bold text-blue-300">Type:</span>
                    {dnsRequest.type}
                </p>
                <p>
                    <span class="font-bold text-blue-300">Class:</span>
                    {dnsRequest.class}
                </p>
            </div>

            <form on:submit|preventDefault={submitAction} class="mt-6">
                <fieldset>
                    <legend
                        class="block text-xl font-semibold mb-4 text-blue-300"
                        >Select an action:</legend
                    >
                    <div class="grid grid-cols-2 gap-4">
                        {#each ["correct", "corrupt", "delay", "nxdomain"] as action}
                            <label
                                class="flex items-center bg-gray-700 p-3 rounded-lg cursor-pointer transition-all duration-200 hover:bg-gray-600"
                            >
                                <input
                                    type="radio"
                                    name="action"
                                    value={action}
                                    bind:group={selectedAction}
                                    class="form-radio h-5 w-5 text-blue-500"
                                />
                                <span class="ml-2 capitalize">{action}</span>
                            </label>
                        {/each}
                    </div>
                </fieldset>
                <button
                    type="submit"
                    class="mt-6 w-full bg-blue-500 text-white px-4 py-2 rounded-lg hover:bg-blue-600 transition-colors duration-200 disabled:opacity-50 disabled:cursor-not-allowed"
                    disabled={!selectedAction || isSubmitting}
                >
                    {isSubmitting ? "Submitting..." : "Submit"}
                </button>
            </form>

            {#if submissionMessage}
                <div
                    in:fly={{ y: 20, duration: 300 }}
                    class="mt-4 text-center text-green-400 font-semibold"
                >
                    {submissionMessage}
                </div>
            {/if}
        </div>
    {:else if errorMessage}
        <div class="text-center mt-10" in:fade>
            <p class="text-xl text-red-400">{errorMessage}</p>
            {#if countdown > 0}
                <p class="text-sm text-gray-400 mt-2">
                    Retrying in {(countdown / 1000).toFixed(3)} seconds...
                </p>
                <div class="w-full bg-gray-700 rounded-full h-2.5 mt-2">
                    <div
                        class="bg-blue-500 h-2.5 rounded-full"
                        style="width: {100 - (countdown / retryDelay) * 100}%"
                    ></div>
                </div>
            {/if}
        </div>
    {/if}
    </div>
</div>

<style>
    /* Add any component-specific styles here */
    input[type="radio"] {
        @apply text-blue-500;
    }
    input[type="radio"]:checked {
        @apply ring-2 ring-blue-500;
    }
</style>
