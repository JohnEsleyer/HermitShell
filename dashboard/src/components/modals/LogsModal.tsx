import { useState, useEffect } from 'react';
import { X, TerminalSquare } from 'lucide-react';
import { Agent } from '../../types';

const API_BASE = '';

interface LogsModalProps {
  agent: Agent;
  onClose: () => void;
}

interface LogEntry {
  id: number;
  agent_id: number;
  user_id: string;
  action: string;
  details: string;
  created_at: string;
}

export function LogsModal({ agent, onClose }: LogsModalProps) {
  const [logs, setLogs] = useState<LogEntry[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const fetchLogs = async () => {
      try {
        const res = await fetch(`${API_BASE}/api/agents/${agent.id}/logs`);
        const data = await res.json();
        setLogs(data);
      } catch (err) {
        console.error('Failed to fetch logs:', err);
      } finally {
        setLoading(false);
      }
    };
    fetchLogs();
  }, [agent.id]);

  return (
    <div className="fixed inset-0 bg-black/85 backdrop-blur-md flex items-center justify-center z-50 p-4 sm:p-6 animate-in fade-in duration-300">
      <div className="bg-zinc-950 border border-zinc-800 w-full max-w-4xl h-[min(82vh,920px)] rounded-[2.5rem] relative flex flex-col shadow-2xl overflow-hidden">
        <div className="p-6 border-b border-zinc-800 flex justify-between items-center bg-zinc-900/50">
          <h2 className="text-2xl font-bold text-white flex items-center gap-3">
            <TerminalSquare className="w-6 h-6" /> Terminal Logs: {agent.name}
          </h2>
          <button onClick={onClose} className="w-10 h-10 bg-zinc-800 rounded-full flex items-center justify-center text-zinc-400 hover:text-white transition-all">
            <X className="w-5 h-5" />
          </button>
        </div>
        <div className="flex-1 bg-[#0a0a0a] p-6 overflow-y-auto font-mono text-sm text-zinc-400 flex flex-col gap-2">
          {loading ? (
            <div>Loading logs...</div>
          ) : logs.length === 0 ? (
            <div className="text-zinc-500">No logs available</div>
          ) : (
            logs.map(log => (
              <div key={log.id} className="p-3 rounded-lg bg-zinc-900/50 border border-zinc-800">
                <div className="text-xs text-zinc-500 mb-1">{log.created_at}</div>
                <div className="text-zinc-300">
                  <span className="text-blue-400">[{log.action}]</span> {log.details || 'No details'}
                </div>
              </div>
            ))
          )}
          <div className="animate-pulse mt-4">_</div>
        </div>
      </div>
    </div>
  );
}
