export interface DockerStatsSnapshot {
  cpu_stats?: {
    cpu_usage?: {
      total_usage?: number;
      percpu_usage?: number[];
    };
    system_cpu_usage?: number;
  };
  precpu_stats?: {
    cpu_usage?: {
      total_usage?: number;
    };
    system_cpu_usage?: number;
  };
  memory_stats?: {
    usage?: number;
    limit?: number;
  };
}

export interface ContainerResourceUsage {
  id: string;
  name: string;
  image: string;
  state: string;
  status: string;
  cpuPercent: number;
  memoryUsageBytes: number;
  memoryLimitBytes: number;
  memoryPercent: number;
}

export interface ContainerResourcesSummary {
  totalCpuPercent: number;
  totalMemoryUsageBytes: number;
  totalMemoryLimitBytes: number;
  totalMemoryPercent: number;
  containers: ContainerResourceUsage[];
}

export interface HostResourceSnapshot {
  totalMemoryBytes: number;
  freeMemoryBytes: number;
  loadAverage: [number, number, number];
  cpuCount: number;
  uptimeSeconds: number;
}

export interface SystemResourcesSummary {
  hostMemoryTotalBytes: number;
  hostMemoryUsedBytes: number;
  hostMemoryPercent: number;
  loadAverage: [number, number, number];
  cpuCount: number;
  uptimeSeconds: number;
  containersCpuPercent: number;
  containersMemoryUsageBytes: number;
  containersMemoryLimitBytes: number;
  containersMemoryPercent: number;
  containerMemoryOfHostPercent: number;
}

export function calculateCpuPercent(stats: DockerStatsSnapshot): number {
  const cpuDelta = (stats.cpu_stats?.cpu_usage?.total_usage || 0) - (stats.precpu_stats?.cpu_usage?.total_usage || 0);
  const systemDelta = (stats.cpu_stats?.system_cpu_usage || 0) - (stats.precpu_stats?.system_cpu_usage || 0);
  const cpuCount = stats.cpu_stats?.cpu_usage?.percpu_usage?.length || 1;

  if (cpuDelta <= 0 || systemDelta <= 0) return 0;

  return (cpuDelta / systemDelta) * cpuCount * 100;
}

export function calculateMemoryUsage(stats: DockerStatsSnapshot): { usageBytes: number; limitBytes: number; percent: number } {
  const usageBytes = stats.memory_stats?.usage || 0;
  const limitBytes = stats.memory_stats?.limit || 0;
  const percent = limitBytes > 0 ? (usageBytes / limitBytes) * 100 : 0;

  return { usageBytes, limitBytes, percent };
}

export function summarizeContainerResources(containers: ContainerResourceUsage[]): ContainerResourcesSummary {
  const totals = containers.reduce(
    (acc, container) => {
      acc.totalCpuPercent += container.cpuPercent;
      acc.totalMemoryUsageBytes += container.memoryUsageBytes;
      acc.totalMemoryLimitBytes += container.memoryLimitBytes;
      return acc;
    },
    { totalCpuPercent: 0, totalMemoryUsageBytes: 0, totalMemoryLimitBytes: 0 }
  );

  const totalMemoryPercent = totals.totalMemoryLimitBytes > 0
    ? (totals.totalMemoryUsageBytes / totals.totalMemoryLimitBytes) * 100
    : 0;

  return {
    ...totals,
    totalMemoryPercent,
    containers
  };
}

export function summarizeSystemResources(host: HostResourceSnapshot, containers: ContainerResourcesSummary): SystemResourcesSummary {
  const hostMemoryUsedBytes = Math.max(0, host.totalMemoryBytes - host.freeMemoryBytes);
  const hostMemoryPercent = host.totalMemoryBytes > 0 ? (hostMemoryUsedBytes / host.totalMemoryBytes) * 100 : 0;
  const containerMemoryOfHostPercent = host.totalMemoryBytes > 0
    ? (containers.totalMemoryUsageBytes / host.totalMemoryBytes) * 100
    : 0;

  return {
    hostMemoryTotalBytes: host.totalMemoryBytes,
    hostMemoryUsedBytes,
    hostMemoryPercent,
    loadAverage: host.loadAverage,
    cpuCount: host.cpuCount,
    uptimeSeconds: host.uptimeSeconds,
    containersCpuPercent: containers.totalCpuPercent,
    containersMemoryUsageBytes: containers.totalMemoryUsageBytes,
    containersMemoryLimitBytes: containers.totalMemoryLimitBytes,
    containersMemoryPercent: containers.totalMemoryPercent,
    containerMemoryOfHostPercent
  };
}
