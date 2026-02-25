import { spawnAgent } from './src/docker';
import { initDb } from './src/db';

async function test() {
    await initDb();
    const result = await spawnAgent({
        agentId: 6,
        agentName: 'Rain',
        agentRole: 'Assistant',
        dockerImage: 'hermit/base:latest',
        userMessage: 'Create a file called testing-telegram-bot.txt in /app/workspace/out/ containing the text "Hello from Agent". Respond only with OK when done.',
        history: [],
        maxTokens: 500
    });
    console.log('Result:', result.output);
    process.exit(0);
}

test().catch(e => {
    console.error(e);
    process.exit(1);
});
