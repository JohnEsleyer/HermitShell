import { describe, it, expect, vi, beforeEach } from 'vitest';
import { processAgentMessage } from '../src/telegram';
import * as docker from '../src/docker';
import * as db from '../src/db';
import * as workspaceDb from '../src/workspace-db';

vi.mock('../src/docker', () => ({
  spawnAgent: vi.fn(),
  docker: {},
  getCubicleStatus: vi.fn(),
  stopCubicle: vi.fn(),
  removeCubicle: vi.fn(),
  listContainers: vi.fn()
}));

vi.mock('../src/db', () => ({
  getAgentByToken: vi.fn(),
  isAllowed: vi.fn(),
  getBudget: vi.fn(),
  updateSpend: vi.fn(),
  canSpend: vi.fn(),
  updateAuditLog: vi.fn(),
  getAgentById: vi.fn(),
  getSetting: vi.fn(),
  getOperator: vi.fn(),
  getActiveMeetings: vi.fn(() => []),
  createMeeting: vi.fn(),
  updateMeetingTranscript: vi.fn(),
  closeMeeting: vi.fn(),
  getAllAgents: vi.fn(),
  createAgentRuntimeLog: vi.fn(),
  getAllSettings: vi.fn(() => ({}))
}));

vi.mock('../src/workspace-db', () => ({
  claimDueCalendarEvents: vi.fn(),
  updateCalendarEvent: vi.fn(),
  getCalendarEvents: vi.fn(() => []),
  createCalendarEvent: vi.fn(() => 101),
  deleteCalendarEvent: vi.fn(),
  initWorkspaceDatabases: vi.fn()
}));

global.fetch = vi.fn(() => Promise.resolve({ json: () => Promise.resolve({ ok: true }) })) as any;

describe('Legacy panelActions are ignored', () => {
  const mockToken = 'test-token';
  const mockChatId = 12345;
  const mockUserId = 12345;

  beforeEach(() => {
    vi.clearAllMocks();
    (db.getAgentByToken as any).mockResolvedValue({
      id: 1,
      name: 'Test Agent',
      role: 'Assistant',
      telegram_token: mockToken,
      require_approval: 0
    });
    (db.canSpend as any).mockResolvedValue(true);
    (db.isAllowed as any).mockResolvedValue(true);
  });

  it('ignores panelActions-only payload', async () => {
    (docker.spawnAgent as any).mockResolvedValue({
      containerId: 'cont-123',
      output: JSON.stringify({ panelActions: ['CALENDAR_LIST'] })
    });

    const result = await processAgentMessage(mockToken, mockChatId, mockUserId, 'Hi');

    expect(workspaceDb.getCalendarEvents).not.toHaveBeenCalled();
    expect(result.output).toContain('panelActions');
  });

  it('keeps deterministic message/action payload behavior', async () => {
    (docker.spawnAgent as any).mockResolvedValue({
      containerId: 'cont-123',
      output: JSON.stringify({ message: 'Done', action: '' })
    });

    const result = await processAgentMessage(mockToken, mockChatId, mockUserId, 'Hi');
    expect(result.output).toBe('Done');
  });
});
