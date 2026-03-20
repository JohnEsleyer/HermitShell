export interface Agent {
  id: number;
  name: string;
  role: string;
  personality?: string;
  provider?: string;
  status: 'running' | 'standby' | 'stopped';
  tunnelUrl?: string;
  profilePic?: string;
  bannerUrl?: string;
  containerId?: string;
  allowedUsers?: string;
  model?: string;
  telegramId?: string;
  telegramToken?: string;
  llmApiCalls?: number;
  contextWindow?: number;
  platform?: string;
}

export interface AgentStats {
  wordCount: number;
  tokenEstimate: number;
  contextWindow: number;
  historyCount: number;
  estimatedCost: number;
  llmApiCalls?: number;
}

export interface ToastMessage {
  id: number;
  message: string;
  type?: 'success' | 'error' | 'info';
}

export interface Skill {
  id: number;
  title: string;
  description: string;
  content: string;
  isCore?: boolean;
}

export interface AppItem {
  id: string;
  name: string;
  agentName: string;
  url: string;
  status: 'running' | 'stopped';
}

export interface ContainerItem {
  id: string;
  name: string;
  agentId?: string;
  agentName: string;
  profilePic?: string;
  status: 'running' | 'stopped' | 'active';
  cpu: number;
  memory: number;
  uptime?: string;
  containerId?: string;
  createdAt?: string;
  updatedAt?: string;
}

export interface CalendarEvent {
  id: number;
  agentId: number;
  agentName?: string;
  date: string;
  time: string;
  prompt: string;
  executed: boolean;
  createdAt?: string;
}

export interface AllowlistEntry {
  id: number;
  telegramUserId: string;
  friendlyName: string;
  notes: string;
  createdAt?: string;
}

export interface HostMetrics {
  cpuPercent: number;
  memoryUsed: number;
  memoryTotal: number;
  memoryFree: number;
  diskUsed: number;
  diskTotal: number;
  diskFree: number;
  memoryPercent: number;
  diskPercent: number;
  timestamp: number;
}

export interface ContainerStats {
  name: string;
  cpuPercent: number;
  memUsageMB: number;
  memLimitMB: number;
  status: string;
}

export interface SystemMetrics {
  host: HostMetrics;
  containers: ContainerStats[];
  tunnelURL?: string;
  tunnelEnabled?: boolean;
}
