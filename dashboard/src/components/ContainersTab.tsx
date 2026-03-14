import { useState, useEffect } from 'react';
import { Box, Trash2, HardDrive, RefreshCw } from 'lucide-react';
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

  const [actionPendingIds, setActionPendingIds] = useState<string[]>([]);

  const handleAction = async (container: ContainerItem, action: 'start' | 'stop') => {
    setActionPendingIds(prev => [...prev, container.id]);
    try {
      const res = await fetch(`${API_BASE}/api/containers/${container.id}/action`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ action }),
      });
      const data = await res.json();
      if (!res.ok) {
        triggerToast(`Failed to ${action} container: ${data.error || 'Unknown error'}`, 'error');
      } else {
        triggerToast(`Container ${action} initiated`, 'success');
        await fetchContainers();
      }
    } catch (err) {
      triggerToast(`Failed to ${action} container`, 'error');
    } finally {
      setTimeout(() => {
        setActionPendingIds(prev => prev.filter(id => id !== container.id));
      }, 1000);
    }
  };

  const terminateContainer = async (container: ContainerItem) => {
    if (!confirm(`PERMANENT DELETION: Are you sure you want to terminate ${container.agentName}'s container? All workspace data in this container will be permanently erased.`)) return;
    try {
      await fetch(`${API_BASE}/api/containers/${container.id}`, {
        method: 'DELETE'
      });
      triggerToast('Container terminated permanently');
      fetchContainers();
    } catch (err) {
      triggerToast('Failed to terminate container', 'error');
    }
  };

  if (loading) {
    return (
      <div className="flex-1 flex flex-col items-center justify-center text-zinc-500">
        <div className="w-16 h-16 rounded-full border border-zinc-800 flex items-center justify-center mb-6 animate-spin">
          <RefreshCw className="w-5 h-5 opacity-20" />
        </div>
        <p className="text-sm font-medium tracking-tight text-zinc-400">Loading containers...</p>
      </div>
    );
  }

  const totalCpu = containers.reduce((acc, c) => acc + c.cpu, 0);
  const totalMem = containers.reduce((acc, c) => acc + c.memory, 0);

  return (
    <div className="flex-1 flex flex-col gap-8 animate-in fade-in duration-500">
      {/* Mini Stats */}
      <div className="flex gap-4">
        <div className="bg-zinc-900/50 border border-zinc-800 rounded-2xl px-6 py-4 flex flex-col">
          <span className="text-[10px] uppercase tracking-widest text-zinc-500 border-b border-zinc-800 pb-2 mb-2">Total Containers</span>
          <span className="text-xl font-bold text-white">{containers.length}</span>
        </div>
        <div className="bg-zinc-900/50 border border-zinc-800 rounded-2xl px-6 py-4 flex flex-col">
          <span className="text-[10px] uppercase tracking-widest text-zinc-500 border-b border-zinc-800 pb-2 mb-2">Overall CPU</span>
          <span className="text-xl font-bold text-white">{totalCpu.toFixed(1)}%</span>
        </div>
        <div className="bg-zinc-900/50 border border-zinc-800 rounded-2xl px-6 py-4 flex flex-col">
          <span className="text-[10px] uppercase tracking-widest text-zinc-500 border-b border-zinc-800 pb-2 mb-2">Overall RAM</span>
          <span className="text-xl font-bold text-white">{totalMem.toFixed(0)} MB</span>
        </div>
      </div>

      {containers.length === 0 ? (
        <div className="flex-1 flex flex-col items-center justify-center text-zinc-500 py-20 bg-zinc-900/20 border border-dashed border-zinc-800 rounded-[2.5rem]">
          <Box className="w-10 h-10 mb-4 opacity-10" />
          <p className="text-lg font-bold text-white">No active containers</p>
          <p className="text-sm">Start an agent to deploy a container.</p>
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-2 xl:grid-cols-3 gap-6">
          {containers.map((container) => (
            <div
              key={container.id}
              className="bg-zinc-950 border border-zinc-800 rounded-[2rem] p-8 flex flex-col hover:border-zinc-700 transition-all duration-300 shadow-xl overflow-hidden relative"
            >
              <div className="flex items-center gap-6 mb-8 relative z-10">
                <div className="w-16 h-16 bg-zinc-900 rounded-2xl flex items-center justify-center text-2xl font-bold border border-zinc-800 overflow-hidden shadow-lg shrink-0">
                  {container.profilePic ? (
                    <img src={container.profilePic} alt={container.agentName} className="w-full h-full object-cover" onError={(e) => {
                      (e.target as HTMLImageElement).src = `https://ui-avatars.com/api/?name=${container.agentName}&background=111&color=fff`;
                    }} />
                  ) : (
                    <span className="text-zinc-600">{container.agentName.charAt(0).toUpperCase()}</span>
                  )}
                </div>
                <div className="flex-1 min-w-0">
                  <h3 className="text-lg font-bold text-white truncate">{container.agentName}</h3>
                  <div className="flex items-center gap-2 mt-1">
                    <span className={`w-2 h-2 rounded-full ${container.status === 'running' ? 'bg-emerald-500 animate-pulse' : 'bg-red-500'}`} />
                    <span className="text-[10px] text-zinc-500 uppercase font-bold tracking-widest">{container.status}</span>
                  </div>
                  <div className="text-[10px] text-zinc-600 font-mono mt-1 opacity-50 truncate">{container.id}</div>
                </div>
              </div>

              <div className="flex flex-col gap-4 mb-8 bg-black/40 p-5 rounded-2xl border border-zinc-800/50">
                <div className="grid grid-cols-2 gap-4">
                  <div>
                    <div className="text-[9px] text-zinc-600 uppercase tracking-[0.2em] mb-1 font-bold">Created</div>
                    <div className="text-[11px] text-zinc-400 font-mono">
                      {container.createdAt ? new Date(container.createdAt).toLocaleDateString() : '---'}
                    </div>
                  </div>
                  <div>
                    <div className="text-[9px] text-zinc-600 uppercase tracking-[0.2em] mb-1 font-bold">Last Active</div>
                    <div className="text-[11px] text-zinc-400 font-mono">
                      {container.updatedAt ? new Date(container.updatedAt).toLocaleTimeString() : 'now'}
                    </div>
                  </div>
                </div>
                <div className="h-px bg-zinc-800/50 w-full" />
                <div className="grid grid-cols-2 gap-4">
                  <div className="flex flex-col gap-1">
                    <div className="text-[9px] text-zinc-600 uppercase tracking-[0.2em] font-bold">CPU Usage</div>
                    <div className="text-sm font-bold text-white">{container.cpu.toFixed(1)}%</div>
                  </div>
                  <div className="flex flex-col gap-1">
                    <div className="text-[9px] text-zinc-600 uppercase tracking-[0.2em] font-bold">Memory</div>
                    <div className="text-sm font-bold text-white">{container.memory.toFixed(0)} MB</div>
                  </div>
                </div>
              </div>

              <div className="mt-auto flex flex-col gap-2">
                <div className="grid grid-cols-2 gap-2">
                  <button
                    onClick={() => openModal('workspace', container)}
                    className="flex-1 h-12 bg-white text-black hover:bg-zinc-200 rounded-xl text-xs font-bold transition-all flex items-center justify-center gap-2 shadow-sm"
                  >
                    <HardDrive className="w-3.5 h-3.5" /> Workspace
                  </button>
                  <div className="flex gap-2">
                    {container.status === 'running' ? (
                      <button
                        onClick={() => handleAction(container, 'stop')}
                        disabled={actionPendingIds.includes(container.id)}
                        className="flex-1 h-12 bg-zinc-900 text-red-500 hover:bg-red-500 hover:text-white rounded-xl text-xs font-bold transition-all flex items-center justify-center border border-zinc-800 disabled:opacity-50"
                      >
                        {actionPendingIds.includes(container.id) ? (
                          <RefreshCw className="w-3.5 h-3.5 animate-spin text-zinc-400" />
                        ) : 'Stop'}
                      </button>
                    ) : (
                      <button
                        onClick={() => handleAction(container, 'start')}
                        disabled={actionPendingIds.includes(container.id)}
                        className="flex-1 h-12 bg-zinc-900 text-emerald-500 hover:bg-emerald-500 hover:text-white rounded-xl text-xs font-bold transition-all flex items-center justify-center border border-zinc-800 disabled:opacity-50"
                      >
                        {actionPendingIds.includes(container.id) ? (
                          <RefreshCw className="w-3.5 h-3.5 animate-spin text-zinc-400" />
                        ) : 'Start'}
                      </button>
                    )}
                    <button
                      onClick={() => handleReset(container)}
                      className="w-12 h-12 bg-zinc-900 text-white hover:bg-white hover:text-black rounded-xl text-xs font-bold transition-all flex items-center justify-center border border-zinc-800"
                      title="Reset Container"
                    >
                      <RefreshCw className="w-3.5 h-3.5" />
                    </button>
                  </div>
                </div>
                <button
                  onClick={() => terminateContainer(container)}
                  className="w-full h-10 rounded-xl text-[10px] font-bold text-red-500/50 hover:text-red-500 hover:bg-red-500/10 transition-all border border-red-500/10 flex items-center justify-center gap-2 uppercase tracking-widest"
                >
                  <Trash2 className="w-3 h-3" /> Terminate Container
                </button>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
