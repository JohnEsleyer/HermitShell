import { describe, it, expect } from 'vitest';
import {
  calculateCpuPercent,
  calculateMemoryUsage,
  summarizeContainerResources,
  summarizeSystemResources
} from '../src/resource-usage';

describe('resource usage calculations', () => {
  it('calculates container CPU percent from docker stats snapshots', () => {
    const percent = calculateCpuPercent(
      {
        cpu_stats: {
          cpu_usage: { total_usage: 2_000_000_000, percpu_usage: [1, 1, 1, 1] },
          system_cpu_usage: 200_000_000_000
        },
        precpu_stats: {
          cpu_usage: { total_usage: 1_000_000_000 },
          system_cpu_usage: 190_000_000_000
        }
      } as any
    );

    expect(percent).toBeCloseTo(40, 2);
  });

  it('calculates memory usage and percent', () => {
    const usage = calculateMemoryUsage({
      memory_stats: {
        usage: 512 * 1024 * 1024,
        limit: 2 * 1024 * 1024 * 1024
      }
    } as any);

    expect(usage.usageBytes).toBe(512 * 1024 * 1024);
    expect(usage.limitBytes).toBe(2 * 1024 * 1024 * 1024);
    expect(usage.percent).toBeCloseTo(25, 2);
  });

  it('summarizes container resources and totals', () => {
    const summary = summarizeContainerResources([
      {
        id: 'abc',
        name: 'cubicle-1',
        image: 'hermit/base',
        state: 'running',
        status: 'Up 3 minutes',
        cpuPercent: 12.5,
        memoryUsageBytes: 200,
        memoryLimitBytes: 1000,
        memoryPercent: 20
      },
      {
        id: 'def',
        name: 'cubicle-2',
        image: 'hermit/base',
        state: 'running',
        status: 'Up 1 minute',
        cpuPercent: 7.5,
        memoryUsageBytes: 100,
        memoryLimitBytes: 500,
        memoryPercent: 20
      }
    ]);

    expect(summary.totalCpuPercent).toBeCloseTo(20, 2);
    expect(summary.totalMemoryUsageBytes).toBe(300);
    expect(summary.totalMemoryLimitBytes).toBe(1500);
    expect(summary.totalMemoryPercent).toBeCloseTo(20, 2);
  });

  it('summarizes host resources including container share', () => {
    const summary = summarizeSystemResources(
      {
        totalMemoryBytes: 1000,
        freeMemoryBytes: 250,
        loadAverage: [1, 0.5, 0.2],
        cpuCount: 4,
        uptimeSeconds: 3600
      },
      {
        totalCpuPercent: 12.5,
        totalMemoryUsageBytes: 300,
        totalMemoryLimitBytes: 1500,
        totalMemoryPercent: 20,
        containers: []
      }
    );

    expect(summary.hostMemoryUsedBytes).toBe(750);
    expect(summary.hostMemoryPercent).toBeCloseTo(75, 2);
    expect(summary.containerMemoryOfHostPercent).toBeCloseTo(30, 2);
    expect(summary.containersCpuPercent).toBeCloseTo(12.5, 2);
  });
});
