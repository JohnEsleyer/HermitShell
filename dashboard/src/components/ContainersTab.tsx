import { useState, useEffect } from 'react';
import { Box, Terminal, Trash2, HardDrive, RefreshCw, Cpu, Database, Activity } from 'lucide-react';
import { ContainerItem } from '../types';

const API_BASE = '';

interface ContainersTabProps {
  openModal: (modal: string, data: any) => void;
  triggerToast: (msg: string, type?: 'success' | 'error' | 'info') => void;
}

export function ContainersTab({ openModal, triggerToast }: ContainersTabProps) {
  const [containers, setContainers] = useState<ContainerItem[]>([]);
  const [loading, setLoading] = useState(true);

  const fetchContainers = async () => {
    try {
      const res = await fetch(`${API_BASE}/api/containers`);
      const data = await res.json();
      setContainers(data || []);
    } catch (err) {
      console.error('Failed to fetch containers:', err);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchContainers();
    const interval = setInterval(fetchContainers, 3000);
    return () => clearInterval(interval);
  }, []);

  const handleReset = async (container: ContainerItem) => {
    if (!container.agentId) {
      triggerToast('Cannot reset: Container not linked to a specific agent', 'error');
      return;
    }
    if (!confirm(`Are you sure you want to reset ${container.agentName}'s container? All volatile data will be lost.`)) return;
    try {
      await fetch(`${API_BASE}/api/agents/${container.agentId}/action`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ action: 'reset' }),
      });
      triggerToast('Container reset initiated');
      fetchContainers();
    } catch (err) {
      triggerToast('Failed to reset container', 'error');
    }
  };

  if (loading) {
    return (
      <div className="flex-1 flex flex-col items-center justify-center text-zinc-500">
        <div className="w-32 h-32 rounded-full border-2 border-dashed border-zinc-800 flex items-center justify-center mb-8 animate-spin duration-[3000ms]">
          <Box className="w-10 h-10 opacity-20" />
        </div>
        <p className="text-xl font-bold tracking-tighter text-zinc-400 animate-pulse">synchronizing with docker engine...</p>
      </div>
    );
  }

  const totalCpu = containers.reduce((acc, c) => acc + c.cpu, 0);
  const totalMem = containers.reduce((acc, c) => acc + c.memory, 0);

  return (
    <div className="flex-1 flex flex-col gap-12 animate-in fade-in duration-700">
      {/* Stats Header */}
      <div className="grid grid-cols-1 md:grid-cols-4 gap-4 px-2">
        <StatCard icon={<Box className="w-5 h-5 text-white" />} label="Active Enclaves" value={containers.length.toString()} color="bg-zinc-900" />
        <StatCard icon={<Cpu className="w-5 h-5 text-emerald-400" />} label="Total CPU Load" value={`${totalCpu.toFixed(1)}%`} color="bg-emerald-950/20" />
        <StatCard icon={<Database className="w-5 h-5 text-blue-400" />} label="Memory Footprint" value={`${totalMem.toFixed(0)} MB`} color="bg-blue-950/20" />
        <StatCard icon={<Activity className="w-5 h-5 text-purple-400" />} label="System Status" value="Healthy" color="bg-purple-950/20" />
      </div>

      {containers.length === 0 ? (
        <div className="flex-1 flex flex-col items-center justify-center text-zinc-500 py-20">
          <div className="w-24 h-24 rounded-full bg-zinc-900 flex items-center justify-center mb-6">
            <Box className="w-8 h-8 opacity-50" />
          </div>
          <p className="text-2xl font-black tracking-tight text-white mb-2">no active isolations</p>
          <p className="text-zinc-500 max-w-xs text-center">deploy an agent to spin up a new secure execution environment</p>
        </div>
      ) : (
        <div className="grid grid-cols-1 lg:grid-cols-2 xl:grid-cols-3 gap-8">
          {containers.map((container, idx) => (
            <div
              key={container.id}
              className="group bg-zinc-950/50 backdrop-blur-xl border border-zinc-800/50 rounded-[3rem] p-10 flex flex-col hover:border-white/20 hover:bg-zinc-900/40 transition-all duration-500 shadow-2xl relative overflow-hidden"
              style={{ animationDelay: `${idx * 100}ms` }}
            >
              {/* Card Glow Effect */}
              <div className="absolute -top-24 -right-24 w-48 h-48 bg-white/5 blur-[100px] rounded-full group-hover:bg-white/10 transition-colors" />

              <div className="flex items-start justify-between mb-10 relative z-10">
                <div className="flex items-center gap-6">
                  <div className="relative">
                    <div className="w-20 h-20 bg-zinc-900 rounded-full flex items-center justify-center text-3xl font-black border border-zinc-700 overflow-hidden shadow-2xl group-hover:scale-105 transition-transform duration-500">
                      {container.profilePic ? (
                        <img src={container.profilePic} alt={container.agentName} className="w-full h-full object-cover" />
                      ) : (
                        <span className="text-white">{container.agentName.charAt(0).toUpperCase()}</span>
                      )}
                    </div>
                    {container.status === 'running' && (
                      <div className="absolute -bottom-1 -right-1 w-6 h-6 bg-black rounded-full border border-zinc-800 flex items-center justify-center p-1">
                        <div className="w-full h-full bg-emerald-500 rounded-full animate-pulse shadow-[0_0_10px_rgba(16,185,129,0.5)]" />
                      </div>
                    )}
                  </div>
                  <div>
                    <h3 className="text-2xl font-black tracking-tighter text-white lowercase group-hover:tracking-tight transition-all duration-500">{container.agentName}</h3>
                    <div className="flex items-center gap-2 mt-2">
                      <span className="text-[10px] bg-zinc-800 text-zinc-400 px-2 py-0.5 rounded font-mono uppercase tracking-widest">{container.id.substring(0, 12)}</span>
                    </div>
                  </div>
                </div>
              </div>

              <div className="grid grid-cols-3 gap-6 mb-10 bg-black/40 p-8 rounded-[2rem] border border-zinc-800/30 relative z-10 group-hover:bg-black/60 transition-colors duration-500">
                <Metric label="CPU" value={`${container.cpu.toFixed(1)}%`} progress={container.cpu} />
                <Metric label="RAM" value={`${container.memory.toFixed(0)}mb`} progress={(container.memory / 1024) * 100} />
                <Metric label="State" value={container.status} status={container.status} />
              </div>

              <div className="mt-auto grid grid-cols-2 gap-4 relative z-10">
                <button
                  onClick={() => openModal('workspace', container)}
                  className="bg-white text-black hover:bg-zinc-200 rounded-full py-5 text-xs font-black transition-all flex items-center justify-center gap-2 shadow-lg hover:-translate-y-1"
                >
                  <HardDrive className="w-4 h-4" /> explore workspace
                </button>
                <button
                  onClick={() => handleReset(container)}
                  className="bg-zinc-900 text-white hover:bg-zinc-800 rounded-full py-5 text-xs font-black transition-all flex items-center justify-center gap-2 border border-zinc-800 shadow-inner hover:-translate-y-1"
                >
                  <RefreshCw className="w-4 h-4 group-hover:rotate-180 transition-transform duration-700" /> reset stack
                </button>
                <button
                  onClick={() => triggerToast('Terminate Agent to permanently shard this container', 'info')}
                  className="col-span-2 bg-red-950/10 hover:bg-red-950/30 text-red-500/60 hover:text-red-400 rounded-full py-4 text-[10px] font-black uppercase tracking-widest transition-all border border-red-900/10 flex items-center justify-center gap-2 mt-2"
                >
                  <Trash2 className="w-3.5 h-3.5" /> destroy isolation
                </button>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

function StatCard({ icon, label, value, color }: { icon: any, label: string, value: string, color: string }) {
  return (
    <div className={`p-6 rounded-[2rem] border border-zinc-800/50 flex items-center gap-5 transition-all hover:border-zinc-700 ${color}`}>
      <div className="w-12 h-12 rounded-2xl bg-black border border-zinc-800 flex items-center justify-center shadow-inner">
        {icon}
      </div>
      <div>
        <div className="text-[10px] font-black uppercase tracking-widest text-zinc-500 mb-1">{label}</div>
        <div className="text-2xl font-black tracking-tighter text-white lowercase">{value}</div>
      </div>
    </div>
  );
}

function Metric({ label, value, progress, status }: { label: string, value: string, progress?: number, status?: string }) {
  return (
    <div className="flex flex-col gap-3">
      <div className="text-[10px] font-black uppercase tracking-[0.2em] text-zinc-500">{label}</div>
      <div className={`text-sm font-black tracking-tight lowercase ${status === 'running' ? 'text-emerald-400' : 'text-white'}`}>{value}</div>
      {progress !== undefined && (
        <div className="h-1 w-full bg-zinc-800 rounded-full overflow-hidden">
          <div
            className={`h-full transition-all duration-1000 ${progress > 80 ? 'bg-red-500' : progress > 50 ? 'bg-yellow-500' : 'bg-white'}`}
            style={{ width: `${Math.min(100, progress)}%` }}
          />
        </div>
      )}
    </div>
  );
}
