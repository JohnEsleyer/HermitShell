import { Agent } from '../types';

interface AgentsTabProps {
  agents: Agent[];
  openModal: (modal: string, agent: Agent) => void;
  triggerToast: (msg: string, type?: 'success' | 'error' | 'info') => void;
  fetchAgents: () => void;
}

export function AgentsTab({ agents, openModal, triggerToast, fetchAgents }: AgentsTabProps) {
  const handleDelete = async (agent: Agent) => {
    if (!confirm(`Delete agent ${agent.name}?`)) return;
    try {
      await fetch(`/api/agents/${agent.id}`, { method: 'DELETE' });
      triggerToast(`Agent ${agent.name} deleted`);
      fetchAgents();
    } catch (err) {
      triggerToast('Failed to delete agent', 'error');
    }
  };

  if (agents.length === 0) {
    return (
      <div className="flex-1 flex flex-col items-center justify-center text-zinc-500">
        <div className="w-24 h-24 rounded-full border-2 border-dashed border-zinc-800 flex items-center justify-center mb-6">
          <svg xmlns="http://www.w3.org/2000/svg" width="32" height="32" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className="opacity-50"><path d="M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="M22 21v-2a4 4 0 0 0-3-3.87"/><path d="M16 3.13a4 4 0 0 1 0 7.75"/></svg>
        </div>
        <p className="text-lg font-medium">No agents deployed yet.</p>
        <p className="text-sm">Click "deploy new agent" to get started.</p>
      </div>
    );
  }

  return (
    <div className="flex-1">
      <div className="grid grid-cols-1 xl:grid-cols-2 2xl:grid-cols-3 gap-6">
        {agents.map(agent => (
          <div key={agent.id} className="bg-black border border-zinc-800 rounded-[2.5rem] overflow-hidden hover:border-zinc-700 transition-colors flex flex-col">
            <div className="h-28 bg-gradient-to-r from-zinc-900 via-zinc-800 to-zinc-900 relative">
              {agent.bannerUrl && (
                <img src={agent.bannerUrl} alt={`${agent.name} banner`} className="w-full h-full object-cover" />
              )}
              <div className="absolute inset-0 bg-black/20" />
            </div>

            <div className="p-8 -mt-8 flex-1 flex flex-col">
              <div className="flex justify-between items-start mb-6">
                <div className="w-16 h-16 bg-zinc-900 rounded-full flex items-center justify-center text-2xl font-black border-2 border-zinc-800 overflow-hidden shadow-lg">
                  {agent.profilePic ? (
                    <img src={agent.profilePic} alt={agent.name} className="w-full h-full object-cover" />
                  ) : (
                    agent.name.charAt(0).toUpperCase()
                  )}
                </div>
                <div className="px-4 py-2 rounded-full text-[10px] font-bold uppercase tracking-widest bg-zinc-900 text-white flex items-center gap-2 border border-zinc-800/50">
                  <div className={`w-2 h-2 rounded-full ${agent.status === 'running' ? 'bg-emerald-400' : agent.status === 'standby' ? 'bg-yellow-400' : 'bg-zinc-600'}`}></div>
                  <span>{agent.status}</span>
                </div>
              </div>
              <h3 className="text-3xl font-bold tracking-tight mb-2 lowercase">{agent.name}</h3>
              <p className="text-sm text-zinc-500 mb-2 lowercase">{agent.role}</p>
              {agent.provider && (
                <p className="text-xs text-zinc-600 mb-4">Provider: {agent.provider}</p>
              )}
              <div className="mt-auto grid grid-cols-2 gap-3">
                <button onClick={() => openModal('testConsole', agent)} className="bg-zinc-900 hover:bg-white hover:text-black text-white rounded-full py-3.5 text-xs font-bold transition-all">chat / test</button>
                <button onClick={() => openModal('skills', agent)} className="bg-zinc-900 hover:bg-white hover:text-black text-white rounded-full py-3.5 text-xs font-bold transition-all">skills</button>
                <button onClick={() => openModal('configure', agent)} className="bg-zinc-900 hover:bg-white hover:text-black text-white rounded-full py-3.5 text-xs font-bold transition-all">configure</button>
                <button onClick={() => openModal('logs', agent)} className="bg-zinc-900 hover:bg-white hover:text-black text-white rounded-full py-3.5 text-xs font-bold transition-all">terminal logs</button>
                <button onClick={() => handleDelete(agent)} className="col-span-2 bg-red-950/30 hover:bg-red-900/50 text-red-400 rounded-full py-3.5 text-xs font-bold transition-all">delete agent</button>
              </div>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
