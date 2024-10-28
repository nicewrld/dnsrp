<script>
    import { TwitterIcon, MailIcon } from 'lucide-svelte';
</script>

<div class="max-w-4xl mx-auto p-6">
    <div class="bg-gray-800 text-white rounded-lg shadow-lg p-8 mb-8">
        <h2 class="text-3xl font-bold mb-4 text-center text-blue-400">DNS Server</h2>
        <p class="text-2xl text-center font-mono">coredns:5983</p>
    </div>

    <div class="bg-gray-800 text-white rounded-lg shadow-lg p-8 mb-8">
        <h2 class="text-3xl font-bold mb-4 text-blue-400">About the Game</h2>
        <p class="mb-4">
            DNS Roleplay is an interactive game where players take on the role of DNS servers,
            making decisions about how to handle DNS requests. Each action you take affects your
            alignment score:
        </p>
        <ul class="list-disc pl-6 mb-4">
            <li><strong>Correct:</strong> Respond with accurate DNS information</li>
            <li><strong>Corrupt:</strong> Return intentionally incorrect data</li>
            <li><strong>Delay:</strong> Introduce artificial latency</li>
            <li><strong>NXDomain:</strong> Claim the domain doesn't exist</li>
        </ul>
        <p>
            Your choices between honest and mischievous behavior will determine whether you become
            known as a Pure or Evil DNS server. Choose wisely!
        </p>
    </div>

    <div class="bg-gray-800 text-white rounded-lg shadow-lg p-8">
        <h2 class="text-3xl font-bold mb-4 text-blue-400">Contact</h2>
        <div class="flex flex-col gap-4">
            <a href="https://twitter.com/spuhghetti" 
               target="_blank" 
               rel="noopener noreferrer" 
               class="flex items-center gap-3 p-3 bg-gray-700 rounded-lg hover:bg-gray-600 transition-colors">
                <TwitterIcon size={24} />
                <span>@spuhghetti</span>
            </a>
            <a href="mailto:h@hhh.hn" 
               class="flex items-center gap-3 p-3 bg-gray-700 rounded-lg hover:bg-gray-600 transition-colors">
                <MailIcon size={24} />
                <span>h@hhh.hn</span>
            </a>
        </div>
    </div>
</div>
