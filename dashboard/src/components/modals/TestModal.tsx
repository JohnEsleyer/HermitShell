import { useState, useRef, useEffect } from 'react';
import { X, Send, Terminal, Zap, Loader2 } from 'lucide-react';
import { Agent } from '../../types';

const API_BASE = '';

interface TestModalProps {
  agent: Agent;
  onClose: () => void;
}

interface LogEntry {
  id: number;
  source: 'telegram' | 'input' | 'system';
  content: string;
}

export function TestModal({ agent, onClose }: TestModalProps) {
  const [input, setInput] = useState('');
  const [chatId, setChatId] = useState('');
  const [allowlist, setAllowlist] = useState<{ friendlyName: string; telegramUserId: string }[]>([]);
  const [logs, setLogs] = useState<LogEntry[]>([
    { id: 1, source: 'system', content: '{\n  "status": "READY",\n  "agent": "' + agent.name + '"\n}' }
  ]);
  const [testingLLM, setTestingLLM] = useState(false);
  const logsEndRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    logsEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [logs]);

  useEffect(() => {
    fetch(`${API_BASE}/api/allowlist`)
      .then(res => res.json())
      .then(data => setAllowlist(data || []))
      .catch(err => console.error('Failed to fetch allowlist:', err));
  }, []);

  const testLLM = async () => {
    setTestingLLM(true);
    setLogs(prev => [...prev, { id: Date.now(), source: 'system', content: 'Testing LLM configuration...' }]);

    try {
      // Create a simple test message
      const testMessage = "Say 'Hello' if you can hear me.";
      
      // Use the test-contract endpoint to test the LLM
      const res = await fetch(`${API_BASE}/api/test-contract`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ 
          payload: `<message>${testMessage}</message>`, 
          userId: 'test-user', 
          agentId: agent.id 
        }),
      });
      const data = await res.json();

      if (res.ok) {
        setLogs(prev => [...prev, { 
          id: Date.now(), 
          source: 'system', 
          content: `✅ LLM Test Successful!\n\nProvider: ${agent.provider}\nModel: ${agent.model}\n\nResponse: ${JSON.stringify(data.actionEffects || data, null, 2)}` 
        }]);
      } else {
        setLogs(prev => [...prev, { 
          id: Date.now(), 
          source: 'system', 
          content: `❌ LLM Test Failed!\n\nError: ${data.error || 'Unknown error'}\n\nProvider: ${agent.provider}\nModel: ${agent.model}` 
        }]);
      }
    } catch (err: any) {
      setLogs(prev => [...prev, { 
        id: Date.now(), 
        source: 'system', 
        content: `❌ LLM Test Failed!\n\nError: ${err.message}\n\nProvider: ${agent.provider}\nModel: ${agent.model}` 
      }]);
    } finally {
      setTestingLLM(false);
    }
  };

  const handleSend = async () => {
    if (!input.trim()) return;

    setLogs(prev => [...prev, { id: Date.now(), source: 'input', content: input }]);

    const userId = chatId || 'test';

    try {
      const res = await fetch(`${API_BASE}/api/test-contract`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ payload: input, userId: userId, agentId: agent.id }),
      });
      const data = await res.json();

      if (data.actionEffects) {
        data.actionEffects.forEach((effect: any) => {
          setLogs(prev => [...prev, {
            id: Date.now() + Math.random(),
            source: 'system',
            content: JSON.stringify(effect, null, 2)
          }]);
        });
      }
    } catch (err) {
      setLogs(prev => [...prev, {
        id: Date.now(),
        source: 'system',
        content: JSON.stringify({ error: 'Failed to process' }, null, 2)
      }]);
    }

    setInput('');
  };

  const insertExample = (xml: string) => {
    setInput(xml);
  };

  return (
    <div className="fixed inset-0 bg-black/85 backdrop-blur-md flex items-center justify-center z-50 p-4 sm:p-6 animate-in fade-in duration-300">
      <div className="bg-zinc-950 border border-zinc-800 w-full max-w-7xl h-[min(88vh,980px)] rounded-[2.5rem] relative flex flex-col shadow-2xl overflow-hidden">

          <div className="p-6 border-b border-zinc-800 flex justify-between items-center bg-zinc-900/50">
          <div>
            <h2 className="text-2xl font-bold text-white flex items-center gap-3">
              <Terminal className="w-6 h-6" /> Test Console: {agent.name}
            </h2>
            <p className="text-sm text-zinc-400 mt-1">Simulate XML inputs or Telegram takeover commands.</p>
          </div>
          <div className="flex gap-2">
            <button 
              onClick={testLLM} 
              disabled={testingLLM}
              className="px-4 py-2 bg-emerald-900/50 hover:bg-emerald-900 border border-emerald-700/50 rounded-lg text-xs font-bold text-emerald-400 transition-colors flex items-center gap-2 disabled:opacity-50"
            >
              {testingLLM ? <Loader2 className="w-4 h-4 animate-spin" /> : <Zap className="w-4 h-4" />}
              {testingLLM ? 'Testing...' : 'Test LLM'}
            </button>
            <button onClick={() => insertExample('<terminal>ls -la</terminal>')} className="px-4 py-2 bg-zinc-800 hover:bg-zinc-700 rounded-lg text-xs font-mono text-zinc-300 transition-colors">Terminal</button>
            <button onClick={() => insertExample('<action type="GIVE">test.txt</action>')} className="px-4 py-2 bg-zinc-800 hover:bg-zinc-700 rounded-lg text-xs font-mono text-zinc-300 transition-colors">Give File</button>
            <button onClick={() => insertExample('<system>memory</system>')} className="px-4 py-2 bg-zinc-800 hover:bg-zinc-700 rounded-lg text-xs font-mono text-zinc-300 transition-colors">System Info</button>
            <button onClick={onClose} className="ml-4 w-10 h-10 bg-zinc-800 rounded-full flex items-center justify-center text-zinc-400 hover:text-white transition-all">
              <X className="w-5 h-5" />
            </button>
          </div>
        </div>

        <div className="flex flex-1 overflow-hidden">
          <div className="w-1/2 border-r border-zinc-800 p-6 overflow-y-auto bg-[#0a0a0a] flex flex-col gap-4">
            {logs.map(log => (
              <div key={log.id} className={`p-4 rounded-xl border font-mono text-sm whitespace-pre-wrap animate-in slide-in-from-top-2 duration-300 ${log.source === 'telegram' ? 'bg-blue-950/30 border-blue-900/50 text-blue-200' :
                log.source === 'input' ? 'bg-yellow-950/30 border-yellow-900/50 text-yellow-200' :
                  'bg-emerald-950/30 border-emerald-900/50 text-emerald-200'
                }`}>
                <div className="text-[10px] uppercase tracking-wider opacity-50 mb-2">{log.source}</div>
                {log.content}
              </div>
            ))}
            <div ref={logsEndRef} />
          </div>

          <div className="w-1/2 p-6 flex flex-col bg-zinc-950">
            <label className="text-sm font-bold text-zinc-400 mb-2 uppercase tracking-wider">Telegram Chat ID (for test sending)</label>
            <select
              value={chatId}
              onChange={e => setChatId(e.target.value)}
              className="mb-4 bg-black border border-zinc-800 rounded-xl px-4 py-2 text-zinc-300 font-mono text-sm outline-none focus:border-zinc-600 appearance-none"
            >
              <option value="">No Telegram (test locally only)</option>
              {allowlist.map(item => (
                <option key={item.telegramUserId} value={item.telegramUserId}>
                  {item.friendlyName} ({item.telegramUserId})
                </option>
              ))}
            </select>
            <label className="text-sm font-bold text-zinc-400 mb-2 uppercase tracking-wider">XML Input</label>
            <textarea
              value={input}
              onChange={e => setInput(e.target.value)}
              className="flex-1 bg-black border border-zinc-800 rounded-2xl p-6 text-zinc-300 font-mono text-sm outline-none focus:border-zinc-600 resize-none"
              placeholder="<terminal>echo 'hello world'</terminal>&#10;<message>Executing command...</message>"
            />
            <div className="mt-6 flex justify-end">
              <button
                onClick={handleSend}
                className="bg-white text-black px-8 py-4 rounded-full font-bold flex items-center gap-2 hover:bg-zinc-200 transition-colors"
              >
                <Send className="w-4 h-4" /> Send to System
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
