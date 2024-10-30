<script>
    import { MailIcon, Github, UserRound } from "lucide-svelte";
</script>

<div class="max-w-4xl mx-auto p-6">
    <div class="bg-gray-800 text-white rounded-lg shadow-lg p-8 mb-8">
        <h2 class="text-3xl font-bold mb-4 text-center text-blue-400">
            DNS Server
        </h2>
        <p class="text-2xl text-center font-mono">128.140.91.136:53</p>
    </div>

    <div class="bg-gray-800 text-white rounded-lg shadow-lg p-8 mb-8">
        <h2 class="text-3xl font-bold mb-4 text-blue-400">About the Game</h2>
        <p class="mb-4">
            act like a dns server but you are a person, you point your machines
            to use the above dns server to give people requests to be able to
            play the game
        </p>
        <ul class="list-disc pl-6 mb-4">
            <li><strong>correct:</strong> be normalcore</li>
            <li><strong>corrupt:</strong> be evilcore (random ip)</li>
            <li><strong>delay:</strong> be slowpilled</li>
            <li><strong>nxdomain:</strong> be gaslight do lying</li>
        </ul>
        <p>leaderboard tracks if you are an evilmaxxer or clearpilled</p>
    </div>

    <div class="bg-gray-800 text-white rounded-lg shadow-lg p-8">
        <h2 class="text-3xl font-bold mb-4 text-blue-400">Contact</h2>
        <div class="flex flex-col gap-4">
            <a
                href="https://github.com/nicewrld"
                target="_blank"
                rel="noopener noreferrer"
                class="flex items-center gap-3 p-3 bg-gray-700 rounded-lg hover:bg-gray-600 transition-colors"
            >
                <Github size={24} />
                <span>@nicewrld</span>
            </a>
            <a
                href="mailto:h@hhh.hn"
                class="flex items-center gap-3 p-3 bg-gray-700 rounded-lg hover:bg-gray-600 transition-colors"
            >
                <MailIcon size={24} />
                <span>h@hhh.hn</span>
            </a>
            <p
                class="flex items-center gap-3 p-3 bg-gray-700 rounded-lg hover:bg-gray-600 transition-colors"
            >
                <UserRound size={24} />
                <span>@zsh on discord</span>
            </p>
        </div>
    </div>
</div>
