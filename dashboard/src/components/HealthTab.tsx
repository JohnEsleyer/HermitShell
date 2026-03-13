import { useState, useEffect, useMemo } from 'react';
import { Cpu, MemoryStick, HardDrive, Network, Box, Activity } from 'lucide-react';
import { SystemMetrics } from '../types';

const API_BASE = '';

const clampPercent = (value: number) => Math.max(0, Math.min(100, value));

const getLevelStyles = (percent: number) => {
  if (percent >= 85) {
    return {
      tone: 'text-red-300',
      bar: 'bg-red-500',
      ring: 'border-red-500/50',
      bg: 'bg-red-500/10'
    };
  }
  if (percent >= 60) {
    return {
      tone: 'text-yellow-300',
      bar: 'bg-yellow-500',
      ring: 'border-yellow-500/50',
      bg: 'bg-yellow-500/10'
    };
  }
  if (percent >= 35) {
    return {
      tone: 'text-blue-300',
      bar: 'bg-blue-500',
      ring: 'border-blue-500/50',
      bg: 'bg-blue-500/10'
    };
  }
  return {
    tone: 'text-emerald-300',
    bar: 'bg-emerald-500',
    ring: 'border-emerald-500/50',
    bg: 'bg-emerald-500/10'
  };
};

export function HealthTab() {
  const [metrics, setMetrics] = useState<SystemMetrics>({
    host: {
      cpuPercent: 0,
      memoryUsed: 0,
      memoryTotal: 0,
      memoryFree: 0,
      diskUsed: 0,
      diskTotal: 0,
      diskFree: 0,
      memoryPercent: 0,
      diskPercent: 0,
      timestamp: 0
    },
    containers: []
  });
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const fetchMetrics = async () => {
      try {
        const res = await fetch(`${API_BASE}/api/metrics`);
        const data = await res.json();
        setMetrics(data);
      } catch (err) {
        console.error('Failed to fetch metrics:', err);
      } finally {
        setLoading(false);
      }
    };

    fetchMetrics();
    const interval = setInterval(fetchMetrics, 2000);
    return () => clearInterval(interval);
  }, []);

  const formatBytes = (bytes: number) => {
    const gb = bytes / (1024 * 1024 * 1024);
    return gb.toFixed(1);
  };

  const hermitUsage = useMemo(() => {
    const cpu = metrics.containers.reduce((sum, c) => sum + c.cpuPercent, 0);
    const memMB = metrics.containers.reduce((sum, c) => sum + c.memUsageMB, 0);
    const memoryTotalMB = metrics.host.memoryTotal / (1024 * 1024);
    const memoryPercent = memoryTotalMB > 0 ? (memMB / memoryTotalMB) * 100 : 0;

    return {
      cpu,
      memMB,
      count: metrics.containers.length,
      cpuPercent: clampPercent(cpu),
      memoryPercent: clampPercent(memoryPercent)
    };
  }, [metrics.containers, metrics.host.memoryTotal]);

  const showSkeleton = loading || metrics.host.timestamp === 0;

  const progress = (percent: number) => {
    const safePercent = clampPercent(percent);
    const styles = getLevelStyles(safePercent);

    return (
      <div className="mt-4">
        <div className="w-full h-2 rounded-full bg-zinc-800 overflow-hidden">
          <div className={`h-full ${styles.bar}`} style={{ width: `${safePercent}%` }} />
        </div>
        <div className="mt-2 text-[11px] text-zinc-500">{safePercent.toFixed(1)}%</div>
      </div>
    );
  };

  const metricCard = (title: string, icon: JSX.Element, percent: number, body: JSX.Element) => {
    const styles = getLevelStyles(percent);
    return (
      <div className={`rounded-3xl p-5 border border-zinc-800 ${styles.ring} ${styles.bg}`}>
        <div className="flex items-center justify-between">
          <span className="text-zinc-300 text-sm lowercase flex items-center gap-2">{icon} {title}</span>
          <span className={`text-xs font-semibold ${styles.tone}`}>{clampPercent(percent).toFixed(0)}%</span>
        </div>
        <div className="mt-4">{body}</div>
        {progress(percent)}
      </div>
    );
  };

  const hostCpuPercent = clampPercent(metrics.host.cpuPercent);
  const hostMemoryPercent = clampPercent(metrics.host.memoryPercent);
  const hostDiskPercent = clampPercent(metrics.host.diskPercent);
  const hermitOverallPercent = clampPercent((hermitUsage.cpuPercent + hermitUsage.memoryPercent) / 2);

  return (
    <div className="flex-1 flex flex-col gap-6">
      <div className="grid grid-cols-1 lg:grid-cols-2 xl:grid-cols-5 gap-4">
        {metricCard('host cpu', <Cpu className="w-4 h-4" />, hostCpuPercent, showSkeleton ? <div className="h-12 rounded-lg bg-zinc-800 animate-pulse" /> : (
          <div className="flex items-baseline gap-2">
            <span className="text-4xl font-black tracking-tighter">{metrics.host.cpuPercent.toFixed(1)}</span><span className="text-lg font-bold text-zinc-500">%</span>
          </div>
        ))}

        {metricCard('host memory', <MemoryStick className="w-4 h-4" />, hostMemoryPercent, showSkeleton ? <div className="h-12 rounded-lg bg-zinc-800 animate-pulse" /> : (
          <>
            <div className="flex items-baseline gap-2">
              <span className="text-4xl font-black tracking-tighter">{formatBytes(metrics.host.memoryUsed)}</span><span className="text-lg font-bold text-zinc-500">gb</span>
            </div>
            <span className="text-xs text-zinc-400 mt-1 block">{formatBytes(metrics.host.memoryFree)} gb free</span>
          </>
        ))}

        {metricCard('storage', <HardDrive className="w-4 h-4" />, hostDiskPercent, showSkeleton ? <div className="h-12 rounded-lg bg-zinc-800 animate-pulse" /> : (
          <>
            <div className="flex items-baseline gap-2">
              <span className="text-4xl font-black tracking-tighter">{formatBytes(metrics.host.diskUsed)}</span><span className="text-lg font-bold text-zinc-500">gb</span>
            </div>
            <span className="text-xs text-zinc-400 mt-1 block">of {formatBytes(metrics.host.diskTotal)} gb total</span>
          </>
        ))}

        {metricCard('hermit workload', <Activity className="w-4 h-4" />, hermitOverallPercent, showSkeleton ? <div className="h-12 rounded-lg bg-zinc-800 animate-pulse" /> : (
          <>
            <div className="flex items-baseline gap-2">
              <span className="text-3xl font-black tracking-tighter">{hermitUsage.cpu.toFixed(1)}</span><span className="text-base font-bold text-zinc-500">cpu %</span>
            </div>
            <span className="text-xs text-zinc-400 mt-1 block">{hermitUsage.memMB.toFixed(0)} MB RAM / {hermitUsage.count} containers</span>
          </>
        ))}

        {metricCard('network', <Network className="w-4 h-4" />, metrics.tunnelURL || metrics.domain ? 20 : 90, showSkeleton ? <div className="h-12 rounded-lg bg-zinc-800 animate-pulse" /> : (
          <div className="flex flex-col gap-1 text-xs">
            <span className={`${metrics.tunnelURL || metrics.domain ? 'text-emerald-300' : 'text-red-300'}`}>{metrics.domainMode ? 'Domain' : metrics.tunnelURL ? 'Tunnel Active' : 'Offline'}</span>
            {metrics.tunnelURL && <div className="text-zinc-400 break-all">{metrics.tunnelURL}</div>}
            {metrics.domain && metrics.domainMode && <div className="text-zinc-400 break-all">https://{metrics.domain}</div>}
          </div>
        ))}
      </div>

      <h3 className="text-2xl font-bold mt-2 mb-2">Docker Containers</h3>
      {metrics.containers.length === 0 ? (
        <div className="bg-black border border-zinc-800 rounded-[2.5rem] p-12 flex flex-col items-center justify-center text-zinc-500">
          <Box className="w-12 h-12 mb-4 opacity-50" />
          <p className="text-lg font-medium">No containers running</p>
          <p className="text-sm">Deploy an agent to start containers</p>
        </div>
      ) : (
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
          {metrics.containers.map((container, idx) => (
            <div key={idx} className="bg-black border border-zinc-800 rounded-3xl p-6 flex items-center justify-between">
              <div className="flex items-center gap-4">
                <div className="w-12 h-12 bg-zinc-900 rounded-full flex items-center justify-center">
                  <Box className="w-6 h-6 text-zinc-400" />
                </div>
                <div>
                  <h4 className="font-bold">{container.name}</h4>
                  <span className="text-xs text-emerald-400">running</span>
                </div>
              </div>
              <div className="flex gap-8 text-right">
                <div>
                  <div className="text-xs text-zinc-500 mb-1">cpu</div>
                  <div className="font-mono">{container.cpuPercent.toFixed(1)}%</div>
                </div>
                <div>
                  <div className="text-xs text-zinc-500 mb-1">mem</div>
                  <div className="font-mono">{container.memUsageMB.toFixed(0)} MB</div>
                </div>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
