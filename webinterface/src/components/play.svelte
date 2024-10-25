<script>
    import { onMount } from "svelte";
    import { fade } from "svelte/transition";
    import { push } from "svelte-spa-router";

    let dnsRequest = null;
    let selectedAction = "";
    let errorMessage = "";
    let submissionMessage = "";
    let countdown = 0;
    let retryDelay = 0;
    let countdownInterval;

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

                // Clear any existing countdown
                clearInterval(countdownInterval);
                countdown = 0;
            } else if (response.status === 401) {
                push("/register");
            } else {
                // Display error and start countdown with ms accuracy
                errorMessage = "No DNS requests available.";
                retryDelay = getRandomInt(1000, 5000); // Random delay between 1000ms and 5000ms
                countdown = retryDelay;
                startRetryCountdown();
            }
        } catch (error) {
            console.error("Error fetching DNS request:", error);
            errorMessage = "Failed to get DNS request.";
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

            // Sleep a random time between 0 and 1 seconds
            const sleepDuration = Math.random() * 1000; // Random duration between 0 and 1000 ms
            await new Promise((resolve) => setTimeout(resolve, sleepDuration));

            await getDNSRequest(); // Try to get a new DNS request
        } else {
            const errorText = await res.text();
            submissionMessage = `Failed to submit action: ${errorText}`;
        }
    }

    onMount(() => {
        getDNSRequest();
        return () => {
            // Cleanup timers on component unmount
            clearInterval(countdownInterval);
        };
    });
</script>

<div class="max-w-2xl mx-auto p-6">
    {#if dnsRequest}
        <div transition:fade>
            <h1 class="text-3xl font-bold mb-4 text-center">
                Handle DNS Request
            </h1>
            <p><strong>Name:</strong> {dnsRequest.name}</p>
            <p><strong>Type:</strong> {dnsRequest.type}</p>
            <p><strong>Class:</strong> {dnsRequest.class}</p>

            <form on:submit|preventDefault={submitAction} class="mt-4">
                <fieldset>
                    <legend class="block text-gray-700 mb-2"
                        >Select an action:</legend
                    >
                    <div class="space-y-2">
                        <label class="flex items-center">
                            <input
                                type="radio"
                                name="action"
                                value="correct"
                                bind:group={selectedAction}
                                required
                                class="form-radio h-4 w-4 text-indigo-600"
                            />
                            <span class="ml-2">Correct</span>
                        </label>
                        <label class="flex items-center">
                            <input
                                type="radio"
                                name="action"
                                value="corrupt"
                                bind:group={selectedAction}
                                class="form-radio h-4 w-4 text-indigo-600"
                            />
                            <span class="ml-2">Corrupt</span>
                        </label>
                        <label class="flex items-center">
                            <input
                                type="radio"
                                name="action"
                                value="delay"
                                bind:group={selectedAction}
                                class="form-radio h-4 w-4 text-indigo-600"
                            />
                            <span class="ml-2">Delay</span>
                        </label>
                        <label class="flex items-center">
                            <input
                                type="radio"
                                name="action"
                                value="nxdomain"
                                bind:group={selectedAction}
                                class="form-radio h-4 w-4 text-indigo-600"
                            />
                            <span class="ml-2">NXDOMAIN</span>
                        </label>
                    </div>
                </fieldset>
                <button
                    type="submit"
                    class="mt-4 bg-indigo-600 text-white px-4 py-2 rounded hover:bg-indigo-700"
                >
                    Submit
                </button>
            </form>

            <!-- Display submission message -->
            {#if submissionMessage}
                <div class="mt-4 text-center text-green-600">
                    {submissionMessage}
                </div>
            {/if}
        </div>
    {:else if errorMessage}
        <div class="text-center mt-10">
            <p class="text-xl text-gray-700">{errorMessage}</p>
            {#if countdown > 0}
                <p class="text-sm text-gray-500">
                    Retrying in {(countdown / 1000).toFixed(3)} seconds...
                </p>
            {/if}
        </div>
    {/if}
</div>
