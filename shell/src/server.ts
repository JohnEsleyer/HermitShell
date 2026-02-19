import { handleTelegramUpdate, sendTelegramMessage } from './telegram';
import { getAllAgents, isAllowed, initDb, getAdminCount, createAdmin, getAdmin } from './db';
import { checkDocker } from './docker';
import { hashPassword, verifyPassword, generateSessionToken } from './auth';
import * as fs from 'fs';
import * as path from 'path';
import cookie from '@fastify/cookie';

const PORT = process.env.PORT || 3000;
const SESSION_SECRET = process.env.SESSION_SECRET || 'hermit-secret-change-in-production';

export async function startServer() {
    await initDb();
    
    const fastify = require('fastify')({ logger: true });

    await fastify.register(cookie);

    const publicRoutes = [
        '/api/auth/status',
        '/api/auth/setup',
        '/api/auth/login',
        '/health',
        '/dashboard',
        '/dashboard/'
    ];

    fastify.addHook('preHandler', async (request: any, reply: any) => {
        if (request.url.startsWith('/webhook/')) return;
        
        if (publicRoutes.includes(request.url)) return;

        if (request.url.startsWith('/dashboard') || request.url === '/') return;

        if (request.url.startsWith('/api/auth')) return;

        const token = request.cookies.hermit_session;
        if (!token) {
            reply.code(401).send({ error: 'Unauthorized' });
            return;
        }
    });

    fastify.get('/api/auth/status', async (request: any, reply: any) => {
        const adminCount = await getAdminCount();
        if (adminCount === 0) {
            return { status: 'setup_required' };
        }
        
        const token = request.cookies.hermit_session;
        if (token) {
            return { status: 'authenticated' };
        }
        
        return { status: 'login_required' };
    });

    fastify.post('/api/auth/setup', async (request: any, reply: any) => {
        const count = await getAdminCount();
        if (count > 0) return reply.code(403).send({ error: 'Setup already completed' });

        const { username, password } = request.body;
        if (!username || !password) return reply.code(400).send({ error: 'Missing credentials' });

        const { hash, salt } = hashPassword(password);
        await createAdmin(username, hash, salt);

        return { message: 'Admin created. Please login.' };
    });

    fastify.post('/api/auth/login', async (request: any, reply: any) => {
        const { username, password } = request.body;
        const admin = await getAdmin(username);

        if (!admin || !verifyPassword(password, admin.password_hash, admin.salt)) {
            return reply.code(401).send({ error: 'Invalid credentials' });
        }

        const token = generateSessionToken(admin.id);
        
        reply.setCookie('hermit_session', token, {
            path: '/',
            httpOnly: true,
            secure: false,
            sameSite: 'strict',
            maxAge: 60 * 60 * 24 * 7
        });

        return { success: true };
    });

    fastify.post('/api/auth/logout', async (request: any, reply: any) => {
        reply.clearCookie('hermit_session');
        return { success: true };
    });

    fastify.get('/health', async () => {
        const dockerOk = await checkDocker();
        return { 
            status: 'ok', 
            timestamp: new Date().toISOString(),
            docker: dockerOk ? 'online' : 'offline'
        };
    });

    fastify.get('/webhook/:token', async (_request: any, _reply: any) => {
        return { message: 'Use POST /webhook/:token for Telegram updates' };
    });

    fastify.post('/webhook/:token', async (request: any, _reply: any) => {
        try {
            const token = request.params.token;
            const update = request.body as any;
            
            if (!update.message) {
                return { ok: true };
            }

            const chatId = update.message.chat.id;
            const response = await handleTelegramUpdate(token, update);

            if (response) {
                await sendTelegramMessage(token, chatId, response);
            }

            return { ok: true };
        } catch (error) {
            console.error('Error handling webhook:', error);
            return { ok: false, error: 'Internal error' };
        }
    });

    fastify.get('/api/agents', async () => {
        return await getAllAgents();
    });

    fastify.get('/api/docker/status', async () => {
        return { ok: await checkDocker() };
    });

    fastify.register(require('@fastify/static'), {
        root: path.join(__dirname, '../dashboard/dist'),
        prefix: '/dashboard/',
    });

    fastify.get('/dashboard', async (_request: any, reply: any) => {
        return reply.sendFile('index.html');
    });

    fastify.get('/dashboard/', async (_request: any, reply: any) => {
        return reply.sendFile('index.html');
    });

    fastify.get('/', async (_request: any, reply: any) => {
        return reply.redirect('/dashboard/');
    });

    await fastify.listen({ port: Number(PORT), host: '0.0.0.0' });
    console.log(`🦀 Shell listening on port ${PORT}`);
    console.log(`📊 Dashboard available at http://localhost:${PORT}/dashboard/`);
}

if (require.main === module) {
    require('dotenv').config();
    startServer().catch(console.error);
}
