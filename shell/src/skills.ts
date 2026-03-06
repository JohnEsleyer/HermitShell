import * as fs from 'fs';
import * as path from 'path';
import * as crypto from 'crypto';

export interface SkillRecord {
    id: string;
    name: string;
    file_name: string;
    created_at: string;
    updated_at: string;
}

const SKILLS_DIR = path.join(__dirname, '../../data/skills');
const INDEX_PATH = path.join(SKILLS_DIR, 'index.json');
const DEFAULT_SKILL_NAME = 'ClawMotion Video Engine';
const DEFAULT_SKILL_CONTENT = `ClawMotion is a high-precision, programmatic video motion engine designed for AI Agents. Create complex video sequences using declarative blueprints and render with 100% parity between browser preview and server export.

✨ Key Features
🏗️ Isomorphic Core: Same Blueprint logic runs in browser and Node.js
🎯 100% Parity: Browser-based export ensures identical preview and output
🧠 AI-Optimized: Declarative manifests easy for LLMs to generate. See LLM.md
🎲 Deterministic Math: Seeded RNG and easing for frame-perfect reproducibility
⚡ GPU-Native: WebCodecs (VideoEncoder) for hardware-accelerated encoding
🎵 Audio-Reactive: FFT-analyzed audio drives visual animations
📦 Modular Exports: @johnesleyer/clawmotion/core, client, server, blueprints
🎞️ Fast Rendering: Skia Canvas + FFmpeg for server-side production

📋 Requirements
# Ubuntu/Debian
sudo apt-get install -y ffmpeg
FFmpeg: Video encoding (included in server pipeline)

🚀 Quick Start
Installation
npm install @johnesleyer/clawmotion
npm run build

Use in Node.js
import { ClawEngine, Clip } from '@johnesleyer/clawmotion/core';
import { MotionFactory } from '@johnesleyer/clawmotion/server';
import { ProBlueprints } from '@johnesleyer/clawmotion/blueprints';

const config = {
    width: 1280,
    height: 720,
    fps: 30,
    duration: 5
};

const clips: Clip[] = [
    {
        id: 'my-clip',
        blueprintId: 'gradient-bg',
        startTick: 0,
        durationTicks: 150,
        props: { color1: '#1a2a6c', color2: '#b21f1f' }
    }
];

const factory = new MotionFactory();
await factory.render(config, clips, './output.mp4');

Run Examples
npx ts-node examples/hello-world.ts
npx ts-node examples/transitions.ts
npx ts-node examples/keyframes.ts

ClawStudio (Visual Editor)
# From any folder - that folder becomes your workspace
clawmotion studio

# Or with explicit workspace
CLAWMOTION_WORKSPACE=/path/to/folder npm run studio

🏗️ Architecture
Browser (Preview): ClawEngine, ClawPlayer, OffscreenCanvas, VideoEncoder (100% parity)
Server (Node.js): MotionFactory, Skia Canvas, FFmpeg Pipeline (fast rendering)
Shared Core: Blueprints, Math, Animator

Render Modes
- Browser (WebCodecs): 100% parity, fast, production exact replica
- Server (Skia): ~99% parity, fastest, quick previews

🎨 The Blueprint Pattern
Blueprints are pure functions that define how to draw.

Built-in Blueprints
- gradient-bg
- text-hero
- floaty-blobs
- glass-card
- vignette
- video

Transitions
Built-in transitions: fade, slide, zoom

Tech Stack
TypeScript, OffscreenCanvas, WebCodecs, Skia Canvas, FFmpeg, esbuild

Project Structure
clawmotion/
├── src/core
├── src/client
├── src/server
├── src/blueprints
├── src/cli
├── examples
└── studio

🤖 AI Integration
See LLM.md for AI agent context, code snippets, and best practices.

⚖️ License
ISC License.`;

function ensureSkillsStore(): void {
    if (!fs.existsSync(SKILLS_DIR)) fs.mkdirSync(SKILLS_DIR, { recursive: true });
    if (!fs.existsSync(INDEX_PATH)) fs.writeFileSync(INDEX_PATH, '[]', 'utf8');
}

function sanitizeFileName(name: string): string {
    return name.trim().toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '').slice(0, 40) || 'skill';
}

function resolveSkillPath(fileName: string): string {
    return path.join(SKILLS_DIR, path.basename(fileName));
}

function safeReadIndex(): SkillRecord[] {
    ensureSkillsStore();

    try {
        const raw = fs.readFileSync(INDEX_PATH, 'utf8');
        const parsed = JSON.parse(raw);
        if (!Array.isArray(parsed)) return [];
        return parsed
            .filter((item) => item && typeof item === 'object')
            .filter((item) => typeof item.id === 'string' && typeof item.file_name === 'string' && typeof item.name === 'string')
            .map((item) => ({
                id: String(item.id),
                name: String(item.name || 'Untitled Skill'),
                file_name: path.basename(String(item.file_name)),
                created_at: String(item.created_at || new Date().toISOString()),
                updated_at: String(item.updated_at || new Date().toISOString())
            }));
    } catch {
        return [];
    }
}

function saveIndex(items: SkillRecord[]): void {
    ensureSkillsStore();
    fs.writeFileSync(INDEX_PATH, JSON.stringify(items, null, 2), 'utf8');
}

function ensureDefaultSkill(items: SkillRecord[]): SkillRecord[] {
    const existing = items.find((item) => item.name === DEFAULT_SKILL_NAME || item.file_name.startsWith('clawmotion-video-engine-'));
    if (existing) {
        const fullPath = resolveSkillPath(existing.file_name);
        if (!fs.existsSync(fullPath)) {
            fs.writeFileSync(fullPath, DEFAULT_SKILL_CONTENT, 'utf8');
        }
        return items;
    }

    const now = new Date().toISOString();
    const skillId = crypto.randomUUID();
    const fileName = `clawmotion-video-engine-${skillId.slice(0, 8)}.md`;
    fs.writeFileSync(resolveSkillPath(fileName), DEFAULT_SKILL_CONTENT, 'utf8');

    return [
        {
            id: skillId,
            name: DEFAULT_SKILL_NAME,
            file_name: fileName,
            created_at: now,
            updated_at: now
        },
        ...items
    ];
}

function loadIndex(): SkillRecord[] {
    const normalized = ensureDefaultSkill(safeReadIndex());
    saveIndex(normalized);
    return normalized;
}

export function listSkills(): Array<SkillRecord & { preview: string }> {
    const items = loadIndex().sort((a, b) => b.updated_at.localeCompare(a.updated_at));
    return items.map((item) => {
        const fullPath = resolveSkillPath(item.file_name);
        const content = fs.existsSync(fullPath) ? fs.readFileSync(fullPath, 'utf8') : '';
        return {
            ...item,
            preview: content.slice(0, 180)
        };
    });
}

export function getSkill(skillId: string): (SkillRecord & { content: string }) | null {
    const skill = loadIndex().find((item) => item.id === skillId);
    if (!skill) return null;

    const fullPath = resolveSkillPath(skill.file_name);
    const content = fs.existsSync(fullPath) ? fs.readFileSync(fullPath, 'utf8') : '';
    return { ...skill, content };
}

export function createSkill(name: string, content: string): SkillRecord {
    const now = new Date().toISOString();
    const skillId = crypto.randomUUID();
    const fileName = `${sanitizeFileName(name)}-${skillId.slice(0, 8)}.md`;

    fs.writeFileSync(resolveSkillPath(fileName), content || '', 'utf8');

    const record: SkillRecord = {
        id: skillId,
        name: name.trim() || 'Untitled Skill',
        file_name: fileName,
        created_at: now,
        updated_at: now
    };

    const index = loadIndex();
    index.push(record);
    saveIndex(index);

    return record;
}

export function updateSkill(skillId: string, updates: { name?: string; content?: string }): SkillRecord | null {
    const index = loadIndex();
    const target = index.find((item) => item.id === skillId);
    if (!target) return null;

    if (typeof updates.name === 'string' && updates.name.trim()) {
        target.name = updates.name.trim();
    }

    if (typeof updates.content === 'string') {
        fs.writeFileSync(resolveSkillPath(target.file_name), updates.content, 'utf8');
    }

    target.updated_at = new Date().toISOString();
    saveIndex(index);
    return target;
}

export function deleteSkill(skillId: string): boolean {
    const index = loadIndex();
    const target = index.find((item) => item.id === skillId);
    if (!target) return false;

    const next = index.filter((item) => item.id !== skillId);
    saveIndex(next);

    const fullPath = resolveSkillPath(target.file_name);
    if (fs.existsSync(fullPath)) fs.unlinkSync(fullPath);

    return true;
}

export function buildSkillsPromptContext(): string {
    const index = loadIndex().sort((a, b) => a.name.localeCompare(b.name));
    if (!index.length) return '';

    const blocks = index.map((skill) => {
        const filePath = resolveSkillPath(skill.file_name);
        const content = fs.existsSync(filePath) ? fs.readFileSync(filePath, 'utf8') : '';
        return `## Skill: ${skill.name}\n${content}`.trim();
    }).filter(Boolean);

    if (!blocks.length) return '';
    return ['Operator Skills (markdown files injected into your context prompt):', ...blocks].join('\n\n');
}
