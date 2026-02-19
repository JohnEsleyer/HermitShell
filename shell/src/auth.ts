import * as crypto from 'crypto';
import { isAllowed } from './db';
import * as fs from 'fs';
import * as path from 'path';

export function hashPassword(password: string): { hash: string, salt: string } {
    const salt = crypto.randomBytes(16).toString('hex');
    const hash = crypto.pbkdf2Sync(password, salt, 1000, 64, 'sha512').toString('hex');
    return { hash, salt };
}

export function verifyPassword(password: string, hash: string, salt: string): boolean {
    const verifyHash = crypto.pbkdf2Sync(password, salt, 1000, 64, 'sha512').toString('hex');
    return hash === verifyHash;
}

export function generateSessionToken(userId: number): string {
    const data = `${userId}:${Date.now()}:${crypto.randomBytes(16).toString('hex')}`;
    return Buffer.from(data).toString('base64');
}

export async function validateUser(userId: number): Promise<boolean> {
    return await isAllowed(userId);
}

interface Limits {
    maxTokensPerRequest: number;
    maxTokensPerDay: number;
    maxHistoryMessages: number;
}

let limits: Limits | null = null;

export function getLimits(): Limits {
    if (limits) return limits;
    
    const configPath = path.join(__dirname, '../../config/limits.json');
    try {
        const data = fs.readFileSync(configPath, 'utf-8');
        limits = JSON.parse(data);
    } catch {
        limits = {
            maxTokensPerRequest: 1000,
            maxTokensPerDay: 10000,
            maxHistoryMessages: 10
        };
    }
    return limits!;
}
