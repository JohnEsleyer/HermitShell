import { createClient, Client } from '@libsql/client';
import * as fs from 'fs';
import * as path from 'path';

const WORKSPACE_DIR = path.join(__dirname, '../../data/workspaces');

function getWorkspacePath(agentId: number, userId: number): string {
    return path.join(WORKSPACE_DIR, `${agentId}_${userId}`, 'data');
}

function getCalendarDbPath(agentId: number, userId: number): string {
    return path.join(getWorkspacePath(agentId, userId), 'calendar.db');
}

interface CalendarEvent {
    id: number;
    agent_id: number;
    title: string;
    prompt: string;
    start_time: string;
    end_time: string | null;
    target_user_id: number;
    color: string | null;
    symbol: string | null;
    recurrence_cron: string | null;
    status: string;
    last_error: string | null;
    started_at: string | null;
    completed_at: string | null;
    created_at: string;
    updated_at: string;
}


interface AgentCalendarRow {
    id: number;
    task_name: string;
    instructions: string;
    scheduled_at: string;
    is_recurring: number;
    cron_expression: string | null;
    status: string;
    last_run_at: string | null;
    next_run_at: string | null;
    created_at: string;
}

function mapAgentCalendarToCalendarEvent(row: AgentCalendarRow, agentId: number, userId: number): CalendarEvent {
    return {
        id: Number(row.id),
        agent_id: agentId,
        title: String(row.task_name || 'Scheduled Task'),
        prompt: String(row.instructions || ''),
        start_time: String(row.scheduled_at || ''),
        end_time: null,
        target_user_id: userId,
        color: null,
        symbol: '⏰',
        recurrence_cron: row.cron_expression || null,
        status: row.status === 'pending' ? 'scheduled' : row.status,
        last_error: null,
        started_at: null,
        completed_at: row.last_run_at || null,
        created_at: String(row.created_at || new Date().toISOString()),
        updated_at: String(row.next_run_at || row.last_run_at || row.created_at || new Date().toISOString())
    };
}

function computeNextRunAt(cronExpression: string, fromDate: Date = new Date()): string | null {
    const parts = String(cronExpression || '').trim().split(/\s+/);
    if (parts.length !== 5) return null;

    const [minRaw, hourRaw, dayRaw, monthRaw, weekRaw] = parts;
    if (dayRaw !== '*' || monthRaw !== '*') return null;

    const minute = Number(minRaw);
    const hour = Number(hourRaw);
    if (Number.isNaN(minute) || Number.isNaN(hour)) return null;

    const base = new Date(fromDate.getTime());
    base.setSeconds(0, 0);

    const candidate = new Date(base.getTime());
    candidate.setMinutes(minute);
    candidate.setHours(hour);

    if (weekRaw === '*') {
        if (candidate <= base) candidate.setDate(candidate.getDate() + 1);
        return candidate.toISOString();
    }

    const targetWeekday = Number(weekRaw);
    if (Number.isNaN(targetWeekday) || targetWeekday < 0 || targetWeekday > 6) return null;

    let daysAhead = (targetWeekday - candidate.getDay() + 7) % 7;
    if (daysAhead === 0 && candidate <= base) daysAhead = 7;
    candidate.setDate(candidate.getDate() + daysAhead);
    return candidate.toISOString();
}

function ensureDataDir(agentId: number, userId: number): void {
    const dataDir = getWorkspacePath(agentId, userId);
    if (!fs.existsSync(dataDir)) {
        fs.mkdirSync(dataDir, { recursive: true });
    }
}

function getCalendarClient(agentId: number, userId: number): Client {
    ensureDataDir(agentId, userId);
    const dbPath = getCalendarDbPath(agentId, userId);
    return createClient({ url: `file:${dbPath}` });
}

export async function initWorkspaceDatabases(agentId: number, userId: number = 0): Promise<void> {
    const calendarClient = getCalendarClient(agentId, userId);

    await calendarClient.execute(`
        CREATE TABLE IF NOT EXISTS calendar_events (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            agent_id INTEGER NOT NULL,
            title TEXT NOT NULL,
            prompt TEXT NOT NULL,
            start_time TEXT NOT NULL,
            end_time TEXT,
            target_user_id INTEGER NOT NULL,
            color TEXT,
            symbol TEXT,
            recurrence_cron TEXT,
            status TEXT DEFAULT 'scheduled',
            last_error TEXT,
            started_at TEXT,
            completed_at TEXT,
            created_at TEXT DEFAULT CURRENT_TIMESTAMP,
            updated_at TEXT DEFAULT CURRENT_TIMESTAMP
        )
    `);

    await calendarClient.execute(`CREATE INDEX IF NOT EXISTS idx_calendar_events_due ON calendar_events(status, start_time)`);
    await calendarClient.execute(`CREATE INDEX IF NOT EXISTS idx_calendar_events_user ON calendar_events(target_user_id, start_time)`);

    await calendarClient.execute(`
        CREATE TABLE IF NOT EXISTS calendar_event_runs (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            event_id INTEGER NOT NULL,
            agent_id INTEGER NOT NULL,
            target_user_id INTEGER NOT NULL,
            run_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
            status TEXT NOT NULL,
            result_message TEXT,
            created_at TEXT DEFAULT CURRENT_TIMESTAMP
        )
    `);

    await calendarClient.execute(`
        CREATE TABLE IF NOT EXISTS agent_calendar (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            task_name TEXT NOT NULL,
            instructions TEXT NOT NULL,
            scheduled_at DATETIME NOT NULL,
            is_recurring BOOLEAN DEFAULT 0,
            cron_expression TEXT,
            status TEXT CHECK(status IN ('pending', 'running', 'completed', 'failed')) DEFAULT 'pending',
            last_run_at DATETIME,
            next_run_at DATETIME,
            created_at DATETIME DEFAULT CURRENT_TIMESTAMP
        )
    `);

    await calendarClient.execute(`
        CREATE TABLE IF NOT EXISTS task_history (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            calendar_id INTEGER,
            executed_at DATETIME DEFAULT CURRENT_TIMESTAMP,
            agent_response TEXT,
            success BOOLEAN,
            FOREIGN KEY (calendar_id) REFERENCES agent_calendar(id)
        )
    `);

    await calendarClient.execute(`
        CREATE INDEX IF NOT EXISTS idx_pending_tasks ON agent_calendar (scheduled_at, status)
    `);
}

export async function createCalendarEvent(event: {
    agent_id: number;
    title: string;
    prompt: string;
    start_time: string;
    end_time?: string | null;
    target_user_id: number;
    color?: string | null;
    symbol?: string | null;
    recurrence_cron?: string | null;
}, userId: number = 0): Promise<number> {
    const client = getCalendarClient(event.agent_id, userId);
    const rs = await client.execute({
        sql: `INSERT INTO calendar_events (agent_id, title, prompt, start_time, end_time, target_user_id, color, symbol, recurrence_cron, status)
              VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 'scheduled')`,
        args: [event.agent_id, event.title, event.prompt, event.start_time, event.end_time || null, event.target_user_id, event.color || null, event.symbol || null, event.recurrence_cron || null]
    });
    return Number(rs.lastInsertRowid);
}

export async function getCalendarEvents(agentId: number, userId: number = 0): Promise<CalendarEvent[]> {
    const client = getCalendarClient(agentId, userId);
    const rs = await client.execute({
        sql: 'SELECT * FROM calendar_events ORDER BY start_time ASC',
        args: []
    });

    const out = rs.rows as unknown as CalendarEvent[];
    try {
        const agentCalendar = await client.execute({
            sql: `SELECT * FROM agent_calendar ORDER BY scheduled_at ASC`,
            args: []
        });
        for (const row of agentCalendar.rows as unknown as AgentCalendarRow[]) {
            out.push(mapAgentCalendarToCalendarEvent(row, agentId, userId));
        }
    } catch {
        // agent_calendar may not exist yet in older workspaces
    }

    return out.sort((a, b) => String(a.start_time).localeCompare(String(b.start_time)));
}

export async function getUpcomingCalendarEvents(agentId: number, userId: number = 0, limit: number = 10): Promise<CalendarEvent[]> {
    const client = getCalendarClient(agentId, userId);
    const now = new Date().toISOString();
    const rs = await client.execute({
        sql: `SELECT * FROM calendar_events 
              WHERE start_time >= ? 
              ORDER BY start_time ASC LIMIT ?`,
        args: [now, limit]
    });
    return rs.rows as unknown as CalendarEvent[];
}

export async function getCalendarEventById(id: number, agentId: number, userId: number = 0): Promise<CalendarEvent | undefined> {
    const client = getCalendarClient(agentId, userId);
    const rs = await client.execute({
        sql: 'SELECT * FROM calendar_events WHERE id = ?',
        args: [id]
    });
    if (rs.rows.length > 0) {
        return rs.rows[0] as unknown as CalendarEvent;
    }
    return undefined;
}

export async function updateCalendarEvent(id: number, agentId: number, updates: Partial<CalendarEvent>, userId: number = 0): Promise<void> {
    const client = getCalendarClient(agentId, userId);
    const fields: string[] = [];
    const values: any[] = [];

    if (updates.title !== undefined) { fields.push('title = ?'); values.push(updates.title); }
    if (updates.prompt !== undefined) { fields.push('prompt = ?'); values.push(updates.prompt); }
    if (updates.start_time !== undefined) { fields.push('start_time = ?'); values.push(updates.start_time); }
    if (updates.end_time !== undefined) { fields.push('end_time = ?'); values.push(updates.end_time); }
    if (updates.target_user_id !== undefined) { fields.push('target_user_id = ?'); values.push(updates.target_user_id); }
    if (updates.color !== undefined) { fields.push('color = ?'); values.push(updates.color); }
    if (updates.symbol !== undefined) { fields.push('symbol = ?'); values.push(updates.symbol); }
    if (updates.status !== undefined) { fields.push('status = ?'); values.push(updates.status); }
    if (updates.recurrence_cron !== undefined) { fields.push('recurrence_cron = ?'); values.push(updates.recurrence_cron); }
    if (updates.last_error !== undefined) { fields.push('last_error = ?'); values.push(updates.last_error); }
    if (updates.started_at !== undefined) { fields.push('started_at = ?'); values.push(updates.started_at); }
    if (updates.completed_at !== undefined) { fields.push('completed_at = ?'); values.push(updates.completed_at); }

    if (!fields.length) return;

    fields.push('updated_at = CURRENT_TIMESTAMP');
    values.push(id);

    await client.execute({
        sql: `UPDATE calendar_events SET ${fields.join(', ')} WHERE id = ?`,
        args: values
    });
}

export async function deleteCalendarEvent(id: number, agentId: number, userId: number = 0): Promise<void> {
    const client = getCalendarClient(agentId, userId);
    await client.execute({ sql: 'DELETE FROM calendar_events WHERE id = ?', args: [id] });
}

export async function claimDueCalendarEvents(agentId: number, userId: number = 0): Promise<CalendarEvent[]> {
    const client = getCalendarClient(agentId, userId);
    const now = new Date().toISOString();
    const rs = await client.execute({
        sql: `SELECT * FROM calendar_events
              WHERE status = 'scheduled'
                AND start_time <= ?
                AND (end_time IS NULL OR end_time >= ?)
              ORDER BY start_time ASC`,
        args: [now, now]
    });

    const due = rs.rows as unknown as CalendarEvent[];
    const claimed: CalendarEvent[] = [];

    for (const event of due) {
        const update = await client.execute({
            sql: `UPDATE calendar_events
                  SET status = 'running', started_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
                  WHERE id = ? AND status = 'scheduled'`,
            args: [event.id]
        });

        if (Number(update.rowsAffected || 0) > 0) {
            claimed.push({ ...event, status: 'running' });
        }
    }

    try {
        const dueAgent = await client.execute({
            sql: `SELECT * FROM agent_calendar
                  WHERE status = 'pending'
                    AND scheduled_at <= ?
                  ORDER BY scheduled_at ASC`,
            args: [now]
        });

        for (const row of dueAgent.rows as unknown as AgentCalendarRow[]) {
            const update = await client.execute({
                sql: `UPDATE agent_calendar SET status = 'running', last_run_at = CURRENT_TIMESTAMP WHERE id = ? AND status = 'pending'`,
                args: [row.id]
            });
            if (Number(update.rowsAffected || 0) <= 0) continue;

            claimed.push({
                ...mapAgentCalendarToCalendarEvent(row, agentId, userId),
                status: 'running'
            });
        }
    } catch {
        // agent_calendar may not exist yet
    }

    return claimed;
}

export function getWorkspaceDataDir(agentId: number, userId: number): string {
    return getWorkspacePath(agentId, userId);
}

export function workspaceDataExists(agentId: number, userId: number): boolean {
    const dataDir = getWorkspacePath(agentId, userId);
    return fs.existsSync(dataDir);
}


export async function completeCalendarTaskHistory(event: CalendarEvent, agentId: number, userId: number, success: boolean, agentResponse: string): Promise<void> {
    const client = getCalendarClient(agentId, userId);

    try {
        const agentCalendar = await client.execute({ sql: 'SELECT * FROM agent_calendar WHERE id = ?', args: [event.id] });
        if (agentCalendar.rows.length > 0) {
            await client.execute({
                sql: 'INSERT INTO task_history (calendar_id, agent_response, success) VALUES (?, ?, ?)',
                args: [event.id, String(agentResponse || '').slice(0, 2000), success ? 1 : 0]
            });

            if (success) {
                const row = agentCalendar.rows[0] as unknown as AgentCalendarRow;
                const recurring = Number(row.is_recurring || 0) === 1 && !!row.cron_expression;
                if (recurring) {
                    const nextRunAt = computeNextRunAt(String(row.cron_expression || ''), new Date());
                    await client.execute({
                        sql: 'UPDATE agent_calendar SET status = ?, next_run_at = ?, scheduled_at = ? WHERE id = ?',
                        args: ['pending', nextRunAt, nextRunAt || row.scheduled_at, event.id]
                    });
                } else {
                    await client.execute({
                        sql: 'UPDATE agent_calendar SET status = ? WHERE id = ?',
                        args: ['completed', event.id]
                    });
                }
            } else {
                await client.execute({
                    sql: 'UPDATE agent_calendar SET status = ? WHERE id = ?',
                    args: ['failed', event.id]
                });
            }
            return;
        }
    } catch {
        // legacy table flow below
    }

    await updateCalendarEvent(event.id, agentId, {
        status: success ? 'completed' : 'failed',
        completed_at: new Date().toISOString(),
        last_error: success ? null : String(agentResponse || '').slice(0, 500)
    }, userId);
}
