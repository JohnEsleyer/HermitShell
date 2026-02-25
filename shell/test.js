const fastify = require('fastify')({ logger: false });
fastify.addHook('preHandler', async (request, reply) => {
    console.log("request.url:", request.url);
    if (request.url.startsWith('/api/internal/')) return;
    reply.code(401).send({ error: 'Unauthorized' });
    return reply;
});
fastify.post('/api/internal/llm', async (request, reply) => { return { ok: 1 }; });
fastify.listen({ port: 3004 }, () => console.log('started'));
