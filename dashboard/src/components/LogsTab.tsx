import { useState, useEffect } from 'react';
import { FileText, Server, Bot, Box, Network, RefreshCw, Search, ChevronDown, ChevronRight, User, Terminal, Globe, Container } from 'lucide-react';

const API_BASE = '';

interface LogEntry {
  id: number;
  agent_id: number;
  user_id: string;
  action: string;
  details: string;
  created_at: string;
}

type LogCategory = 'all' | 'system' | 'agent' | 'docker' | 'network';

export function LogsTab() {
  const [logs, setLogs] = useState<LogEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [category, setCategory] = useState<LogCategory>('all');
  const [search, setSearch] = useState('');
  const [expandedLogs, setExpandedLogs] = useState<Set<number>>(new Set());

  const fetchLogs = async () => {
    try {
      const res = await fetch(`${API_BASE}/api/logs?category=${category}&limit=200`);
      const data = await res.json();
      setLogs(data || []);
    } catch (err) {
      console.error('Failed to fetch logs:', err);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchLogs();
    const interval = setInterval(fetchLogs, 5000);
    return () => clearInterval(interval);
  }, [category]);

  const categories: { id: LogCategory; name: string; icon: typeof FileText }[] = [
    { id: 'all', name: 'All', icon: FileText },
    { id: 'system', name: 'System', icon: Server },
    { id: 'agent', name: 'Agent', icon: Bot },
    { id: 'docker', name: 'Docker', icon: Box },
    { id: 'network', name: 'Network', icon: Network },
  ];

  const getCategoryColor = (action: string | undefined) => {
    if (!action) return 'text-zinc-400 border-zinc-500/30 bg-zinc-500/10';
    if (action.startsWith('system')) return 'text-emerald-400 border-emerald-500/30 bg-emerald-500/10';
    if (action.startsWith('docker')) return 'text-blue-400 border-blue-500/30 bg-blue-500/10';
    if (action.startsWith('tunnel') || action.startsWith('network')) return 'text-purple-400 border-purple-500/30 bg-purple-500/10';
    return 'text-yellow-400 border-yellow-500/30 bg-yellow-500/10';
  };

  const filteredLogs = search
    ? logs.filter(log => 
        (log.action?.toLowerCase() || '').includes(search.toLowerCase()) ||
        (log.details?.toLowerCase() || '').includes(search.toLowerCase()) ||
        (log.user_id?.toLowerCase() || '').includes(search.toLowerCase())
      )
    : logs;

  const toggleExpand = (id: number) => {
    setExpandedLogs(prev => {
      const next = new Set(prev);
      if (next.has(id)) {
        next.delete(id);
      } else {
        next.add(id);
      }
      return next;
    });
  };

  const truncateDetails = (details: string, maxLength = 120) => {
    if (!details) return '';
    if (details.length <= maxLength) return details;
    return details.substring(0, maxLength) + '...';
  };

  const getAgentIcon = (action: string) => {
    if (!action) return <FileText className="w-3 h-3" />;
    if (action.includes('terminal') || action.includes('command')) return <Terminal className="w-3 h-3" />;
    if (action.includes('message') || action.includes('llm')) return <Bot className="w-3 h-3" />;
    if (action.includes('container') || action.includes('docker')) return <Container className="w-3 h-3" />;
    if (action.includes('network') || action.includes('llm_request') || action.includes('tunnel')) return <Globe className="w-3 h-3" />;
    if (action.includes('user') || action.includes('telegram')) return <User className="w-3 h-3" />;
    return <Server className="w-3 h-3" />;
  };

  return (
    <div className="flex-1 flex flex-col gap-6 animate-in fade-in duration-500">
      <div className="flex items-center gap-4 flex-wrap">
        <div className="flex gap-2 bg-zinc-900/50 p-1.5 rounded-2xl border border-zinc-800">
          {categories.map(cat => {
            const Icon = cat.icon;
            return (
              <button
                key={cat.id}
                onClick={() => setCategory(cat.id)}
                className={`flex items-center gap-2 px-4 py-2.5 rounded-xl text-xs font-bold transition-all ${
                  category === cat.id
                    ? 'bg-white text-black'
                    : 'text-zinc-400 hover:text-white hover:bg-zinc-800'
                }`}
              >
                <Icon className="w-4 h-4" />
                {cat.name}
              </button>
            );
          })}
        </div>
        
        <div className="flex-1 relative">
          <Search className="w-4 h-4 absolute left-4 top-1/2 -translate-y-1/2 text-zinc-500" />
          <input
            type="text"
            placeholder="Search logs..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="w-full max-w-md bg-zinc-900/50 border border-zinc-800 rounded-full px-10 py-2.5 text-sm text-white placeholder-zinc-500 outline-none focus:border-zinc-600"
          />
        </div>

        <button
          onClick={fetchLogs}
          className="p-2.5 bg-zinc-900 border border-zinc-800 rounded-full text-zinc-400 hover:text-white hover:bg-zinc-800 transition-all"
        >
          <RefreshCw className="w-4 h-4" />
        </button>
      </div>

      {loading ? (
        <div className="flex-1 flex flex-col items-center justify-center text-zinc-500 py-20">
          <div className="w-16 h-16 rounded-full border border-zinc-800 flex items-center justify-center mb-6 animate-spin">
            <RefreshCw className="w-5 h-5 opacity-20" />
          </div>
          <p className="text-sm font-medium tracking-tight text-zinc-400">Loading logs...</p>
        </div>
      ) : filteredLogs.length === 0 ? (
        <div className="flex-1 flex flex-col items-center justify-center text-zinc-500 py-20 bg-zinc-900/20 border border-dashed border-zinc-800 rounded-[2.5rem]">
          <FileText className="w-10 h-10 mb-4 opacity-10" />
          <p className="text-lg font-bold text-white">No logs found</p>
          <p className="text-sm">Logs will appear here as events occur</p>
        </div>
      ) : (
        <div className="flex-1 overflow-y-auto space-y-2 pr-2">
          {filteredLogs.map((log) => {
            const isExpanded = expandedLogs.has(log.id);
            const truncatedDetails = truncateDetails(log.details);
            const isTruncated = log.details && log.details.length > 120;
            
            return (
              <div
                key={log.id}
                className={`p-4 rounded-2xl border ${getCategoryColor(log.action)} transition-all ${isExpanded ? 'ring-1 ring-white/20' : ''}`}
              >
                <div className="flex items-center justify-between mb-2">
                  <div className="flex items-center gap-3">
                    {getAgentIcon(log.action)}
                    <span className="text-[10px] uppercase font-bold tracking-wider opacity-70">
                      {log.action}
                    </span>
                    {log.agent_id > 0 && (
                      <span className="text-[9px] px-2 py-0.5 rounded bg-zinc-800/50 text-zinc-400">
                        Agent: {log.agent_id}
                      </span>
                    )}
                  </div>
                  <div className="flex items-center gap-2">
                    {isTruncated && (
                      <button
                        onClick={() => toggleExpand(log.id)}
                        className="p-1 hover:bg-zinc-800 rounded transition-colors"
                      >
                        {isExpanded ? (
                          <ChevronDown className="w-4 h-4 opacity-50" />
                        ) : (
                          <ChevronRight className="w-4 h-4 opacity-50" />
                        )}
                      </button>
                    )}
                    <span className="text-[10px] opacity-50 font-mono">
                      {log.created_at ? new Date(log.created_at).toLocaleString() : ''}
                    </span>
                  </div>
                </div>
                <div className={`text-sm font-mono opacity-80 whitespace-pre-wrap ${!isExpanded ? 'line-clamp-3' : ''}`} style={!isExpanded ? { display: '-webkit-box', WebkitLineClamp: 3, WebkitBoxOrient: 'vertical', overflow: 'hidden' } : {}}>
                  {isExpanded ? (log.details || <span className="italic opacity-50">No details</span>) : (truncatedDetails || <span className="italic opacity-50">No details</span>)}
                </div>
                <div className="flex items-center gap-4 mt-2">
                  {log.user_id && (
                    <div className="text-[10px] opacity-50 flex items-center gap-1">
                      <User className="w-3 h-3" />
                      User: {log.user_id}
                    </div>
                  )}
                  {isTruncated && !isExpanded && (
                    <button 
                      onClick={() => toggleExpand(log.id)}
                      className="text-[10px] text-blue-400 hover:text-blue-300"
                    >
                      Show more...
                    </button>
                  )}
                </div>
              </div>
            );
          })}
        </div>
      )}

      <div className="text-xs text-zinc-500 text-center">
        Showing {filteredLogs.length} logs • Auto-refreshes every 5s
      </div>
    </div>
  );
}
