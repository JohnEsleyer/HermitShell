import React, { useState, useEffect } from 'react';
import { LayoutGrid, ExternalLink, Globe, User, Clock, Trash2 } from 'lucide-react';
import { Agent } from '../types';

interface AppInfo {
  name: string;
  url: string;
}

interface AppsTabProps {
  triggerToast: (msg: string, type?: 'success' | 'error' | 'info') => void;
  agents: Agent[];
}

export function AppsTab({ triggerToast, agents }: AppsTabProps) {
  const [agentApps, setAgentApps] = useState<Record<number, AppInfo[]>>({});
  const [loading, setLoading] = useState<Record<number, boolean>>({});

  const runningAgents = agents.filter(a => a.status === 'running');

  useEffect(() => {
    runningAgents.forEach(agent => {
      if (!agentApps[agent.id] && !loading[agent.id]) {
        fetchApps(agent.id);
      }
    });
  }, [runningAgents]);

  const fetchApps = async (agentId: number) => {
    setLoading(prev => ({ ...prev, [agentId]: true }));
    try {
      const resp = await fetch(`/api/agents/${agentId}/apps`);
      if (resp.ok) {
        const data = await resp.json();
        setAgentApps(prev => ({ ...prev, [agentId]: data }));
      }
    } catch (err) {
      console.error(`Failed to fetch apps for agent ${agentId}:`, err);
    } finally {
      setLoading(prev => ({ ...prev, [agentId]: false }));
    }
  };

  const deleteApp = async (agentId: number, appName: string) => {
    if (!confirm(`Delete app "${appName}"? This cannot be undone.`)) return;
    try {
      const resp = await fetch(`/api/agents/${agentId}/apps/${appName}`, {
        method: 'DELETE',
      });
      if (resp.ok) {
        setAgentApps(prev => ({
          ...prev,
          [agentId]: (prev[agentId] || []).filter(a => a.name !== appName)
        }));
        triggerToast(`Deleted ${appName}`, 'success');
      } else {
        triggerToast(`Failed to delete ${appName}`, 'error');
      }
    } catch (err) {
      console.error(`Failed to delete app ${appName}:`, err);
      triggerToast(`Failed to delete ${appName}`, 'error');
    }
  };

  if (runningAgents.length === 0) {
    return (
      <div className="flex-1 flex flex-col items-center justify-center text-zinc-500">
        <div className="w-24 h-24 rounded-full border-2 border-dashed border-zinc-800 flex items-center justify-center mb-6">
          <LayoutGrid className="w-8 h-8 opacity-50" />
        </div>
        <p className="text-lg font-medium">No apps published yet.</p>
        <p className="text-sm">Deploy an agent to start building apps.</p>
      </div>
    );
  }

  // Flatten all apps to show them in a single grid or grouped by agent
  const allApps = runningAgents.flatMap(agent =>
    (agentApps[agent.id] || []).map(app => ({
      ...app,
      agentId: agent.id,
      agentName: agent.name,
      agentPic: agent.profilePic
    }))
  );

  if (allApps.length === 0) {
    return (
      <div className="flex-1 flex flex-col items-center justify-center text-zinc-500">
        <div className="w-24 h-24 rounded-full border-2 border-dashed border-zinc-800 flex items-center justify-center mb-6">
          <Globe className="w-8 h-8 opacity-50" />
        </div>
        <p className="text-lg font-medium">No apps created yet.</p>
        <p className="text-sm">Ask an agent to create a web app using the &lt;app&gt; tag.</p>
      </div>
    );
  }

  return (
    <div className="flex-1">
      <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4 gap-6">
        {allApps.map((app, index) => (
          <div key={`${app.agentId}-${app.name}-${index}`} className="bg-zinc-900/50 border border-zinc-800 rounded-3xl p-6 flex flex-col group hover:border-zinc-700 hover:bg-zinc-900 transition-all duration-300">
            <div className="flex items-start justify-between mb-6">
              <div className="w-12 h-12 bg-white/5 rounded-2xl flex items-center justify-center border border-white/10 group-hover:scale-110 transition-transform">
                <LayoutGrid className="w-6 h-6 text-zinc-400" />
              </div>
              <div className="flex gap-2">
                <button
                  onClick={() => {
                    window.open(app.url, '_blank');
                    triggerToast(`Opening ${app.name}...`, 'info');
                  }}
                  className="w-10 h-10 rounded-full bg-zinc-800 flex items-center justify-center hover:bg-white hover:text-black transition-colors"
                  title="Open App"
                >
                  <ExternalLink className="w-4 h-4" />
                </button>
                <button
                  onClick={() => deleteApp(app.agentId, app.name)}
                  className="w-10 h-10 rounded-full bg-zinc-800 flex items-center justify-center hover:bg-red-500 hover:text-white transition-colors"
                  title="Delete App"
                >
                  <Trash2 className="w-4 h-4" />
                </button>
              </div>
            </div>

            <h3 className="text-xl font-bold text-white mb-2 truncate" title={app.name}>{app.name}</h3>

            <div className="flex items-center gap-2 mb-6">
              {app.agentPic ? (
                <img src={app.agentPic} className="w-5 h-5 rounded-full" alt="" />
              ) : (
                <User className="w-5 h-5 text-zinc-600" />
              )}
              <span className="text-sm text-zinc-400 font-medium">{app.agentName}</span>
            </div>

            <div className="mt-auto pt-6 border-t border-zinc-800 flex items-center justify-between">
              <div className="flex items-center gap-1.5 text-xs text-zinc-500">
                <Clock className="w-3.5 h-3.5" />
                <span>Just now</span>
              </div>
              <span className="px-2.5 py-1 rounded-full bg-emerald-500/10 text-emerald-400 text-[10px] font-bold uppercase tracking-wider">Active</span>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
