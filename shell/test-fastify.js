const fastify = require('fastify')();
fastify.addHook('preHandler', async (req, reply) => {
  console.log('url:', req.url);
  if (req.url.startsWith('/api/internal/')) return;
  reply.code(401).send({error: 'Unauthorized'});
});
fastify.post('/api/internal/llm', async () => ({ok:1}));
fastify.listen({port:3005}, () => console.log('started'))
